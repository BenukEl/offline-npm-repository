package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
)

// MetadataWorkerPool defines the interface for metadata retrieval workers.
type MetadataWorkerPool interface {
	StartWorker(ctx context.Context, analyzeChan chan entities.RetrievePackage, downloadChan chan entities.NpmPackage, workerID int, inactivityTime time.Duration)
	WaitAllWorkers()
}

// metadataWorkerPool is the concrete implementation of MetadataWorker.
type metadataWorkerPool struct {
	logger        logger.Logger
	wg            *sync.WaitGroup
	localNpmRepo  repositories.LocalNpmRepository
	remoteNpmRepo repositories.NpmRepository
	localNpmState entities.LocalNpmState
}

// NewMetadataWorkerPool creates a new instance of MetadataWorker.
func NewMetadataWorkerPool(logger logger.Logger, localNpmRepo repositories.LocalNpmRepository, remoteNpmRepo repositories.NpmRepository, localNpmState entities.LocalNpmState) MetadataWorkerPool {
	return &metadataWorkerPool{
		logger:        logger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  localNpmRepo,
		remoteNpmRepo: remoteNpmRepo,
		localNpmState: localNpmState,
	}
}

// StartWorker runs a metadata retrieval worker.
func (f *metadataWorkerPool) StartWorker(ctx context.Context, analyzeChan chan entities.RetrievePackage, downloadChan chan entities.NpmPackage, workerID int, inactivityTime time.Duration) {
	f.wg.Add(1)
	go func(id int) {
		defer f.wg.Done()

		f.logger.Debug("[meta_#%d] Worker started", id)

		// Create a timer to stop the worker if it is inactive for a certain time.
		timer := time.NewTimer(inactivityTime)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				f.logger.Debug("[meta_#%d] Received context cancellation", id)
				return
			case pkg, ok := <-analyzeChan:
				if !ok {
					f.logger.Debug("[meta_#%d] Metadata channel closed", id)
					return
				}
				if pkg.Name == "" {
					continue
				}
				if err := f.retrieveMetadata(ctx, pkg, analyzeChan, downloadChan, id); err != nil {
					f.logger.Error("[meta_#%d] Failed to retrieve metadata for %s: %w", id, pkg, err)
				}
				// Reset the timer to avoid stopping the worker due to inactivity.
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(inactivityTime)
			case <-timer.C:
				f.logger.Debug("[meta_#%d] Worker stopped due to inactivity", id)
				return
			}
		}
	}(workerID)
}

// WaitAllWorkers waits for all workers to finish.
func (f *metadataWorkerPool) WaitAllWorkers() {
	f.wg.Wait()
}

func (f *metadataWorkerPool) retrieveMetadata(ctx context.Context, pkg entities.RetrievePackage, analyzeChan chan entities.RetrievePackage, downloadChan chan entities.NpmPackage, workerID int) error {
	// Avoid processing the package if already processed.
	if !f.localNpmState.IsAnalysisNeeded(pkg.Name) {
		f.logger.Debug("Package %s already processed", pkg.Name)
		return nil
	}

	f.logger.Debug("[meta_#%d] Fetching metadata for package: %s", workerID, pkg.Name)
	reader, err := f.remoteNpmRepo.FetchMetadata(ctx, pkg.Name)
	if err != nil {
		f.logger.Error("Failed to fetch metadata for %s: %w", pkg.Name, err)
		return err
	}
	defer reader.Close()

	teeReader, err := f.localNpmRepo.WritePackageJSON(pkg.Name, reader)
	if err != nil {
		return fmt.Errorf("failed to write package.json for package %s. Err: %w", pkg.Name, err)
	}
	defer teeReader.Close()

	packages, err := f.remoteNpmRepo.DecodeNpmPackages(teeReader)
	if err != nil {
		return fmt.Errorf("failed to decode npm packages for package %s. Err: %w", pkg.Name, err)
	}

	filteredPackages := f.filterPackages(packages, pkg)

	for _, pkg := range filteredPackages {

		if f.logger.IsDebug() {
			f.logger.Debug("[meta_#%d] Enqueueing package %s:%s for download", workerID, pkg.Name, pkg.Version.String())
		}

		downloadChan <- pkg

		// Enqueue dependencies and peer dependencies for metadata retrieval.
		for _, dep := range pkg.Dependencies {
			if !f.localNpmState.IsAnalysisStarted(dep) {
				f.localNpmState.SetState(dep, entities.AnalysingState)
				analyzeChan <- entities.NewRetrievePackage(dep)
			}
		}
		for _, dep := range pkg.PeerDeps {
			if !f.localNpmState.IsAnalysisStarted(dep) {
				f.localNpmState.SetState(dep, entities.AnalysingState)
				analyzeChan <- entities.NewRetrievePackage(dep)
			}
		}
	}

	f.logger.Debug("[meta_#%d] Processed package %s... %d versions to download", workerID, pkg.Name, len(filteredPackages))
	f.localNpmState.SetState(pkg.Name, entities.AnalysedState)
	f.localNpmState.IncrementAnalysedCount()

	return nil
}

// filterPackages filtre pre-release versions and versions that are less than the last version.
func (f *metadataWorkerPool) filterPackages(npmPackages []entities.NpmPackage, retrievePkg entities.RetrievePackage) []entities.NpmPackage {
	lastSyncDate := f.localNpmState.GetLastSync(retrievePkg.Name)
	var filtered []entities.NpmPackage
	for _, npmPkg := range npmPackages {
		if npmPkg.Version.IsPreRelease() && !retrievePkg.IsMatchingPreRelease(npmPkg.Version.PreRelease) {
			continue // Exclude pre-release versions
		}
		if npmPkg.ReleaseDate.After(lastSyncDate) {
			filtered = append(filtered, npmPkg)
		}
	}
	return filtered
}
