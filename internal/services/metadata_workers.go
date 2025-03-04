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

const maxMetadataRetries = 5

// MetadataWorkerPool defines the interface for metadata retrieval workers.
type MetadataWorkerPool interface {
	StartWorker(ctx context.Context, analyzeChan chan entities.RetrievePackage, downloadChan chan entities.NpmPackage, workerID int, inactivityTime time.Duration)
	WaitAllWorkers()
}

// metadataWorkerPool is the concrete implementation of MetadataWorker.
type metadataWorkerPool struct {
	logger             logger.Logger
	wg                 *sync.WaitGroup
	localNpmRepo       repositories.LocalNpmRepository
	remoteNpmRepo      repositories.NpmRepository
	localNpmState      entities.LocalNpmState
	backoffFactor      time.Duration
	maxDownloadRetries int
}

// NewMetadataWorkerPool creates a new instance of MetadataWorker.
func NewMetadataWorkerPool(logger logger.Logger, localNpmRepo repositories.LocalNpmRepository, remoteNpmRepo repositories.NpmRepository, localNpmState entities.LocalNpmState) MetadataWorkerPool {
	return &metadataWorkerPool{
		logger:             logger,
		wg:                 &sync.WaitGroup{},
		localNpmRepo:       localNpmRepo,
		remoteNpmRepo:      remoteNpmRepo,
		localNpmState:      localNpmState,
		backoffFactor:      time.Second,
		maxDownloadRetries: maxMetadataRetries,
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
	if !f.localNpmState.IsAnalysisNeeded(pkg) {
		f.logger.Debug("Package %s already processed", pkg.Name)
		return nil
	}

	packages, err := f.fetchMetadata(ctx, pkg, workerID)
	if err != nil {
		return err
	}

	filteredPackages := f.filterPackages(packages, pkg)

	for _, pkg := range filteredPackages {

		if f.logger.IsDebug() {
			f.logger.Debug("[meta_#%d] Enqueueing package %s:%s for download", workerID, pkg.Name, pkg.Version.String())
		}

		downloadChan <- pkg

		// Enqueue dependencies and peer dependencies for metadata retrieval.
		for _, dep := range pkg.Dependencies {
			depPkg := entities.NewRetrievePackage(dep)
			if !f.localNpmState.IsAnalysisStarted(depPkg) {
				f.localNpmState.SetState(depPkg, entities.AnalysingState)
				analyzeChan <- depPkg
			}
		}
		for _, dep := range pkg.PeerDeps {
			depPkg := entities.NewRetrievePackage(dep)
			if !f.localNpmState.IsAnalysisStarted(depPkg) {
				f.localNpmState.SetState(depPkg, entities.AnalysingState)
				analyzeChan <- depPkg
			}
		}
	}

	f.logger.Debug("[meta_#%d] Processed package %s... %d versions to download", workerID, pkg.Name, len(filteredPackages))
	f.localNpmState.SetState(pkg, entities.AnalysedState)
	f.localNpmState.IncrementAnalysedCount()

	return nil
}

// fetchMetadata retrieves the metadata for the given package.
func (f *metadataWorkerPool) fetchMetadata(ctx context.Context, pkg entities.RetrievePackage, workerID int) ([]entities.NpmPackage, error) {
	var lastErr error

	for attempt := 1; attempt <= f.maxDownloadRetries; attempt++ {
		if f.logger.IsDebug() {
			f.logger.Debug("[meta_#%d] Attempt %d: Fetching metadata for package %s", workerID, attempt, pkg.Name)
		}

		reader, err := f.remoteNpmRepo.FetchMetadata(ctx, pkg.Name)
		if err != nil {
			f.logger.Error("[meta_#%d] Attempt %d: Failed to fetch metadata for %s. Err: %v", workerID, attempt, pkg.Name, err)
			lastErr = err
			// Progressive delay before the next
			time.Sleep(time.Duration(attempt) * f.backoffFactor)
			continue
		}

		teeReader, err := f.localNpmRepo.WritePackageJSON(pkg.Name, reader)
		reader.Close()
		if err != nil {
			f.logger.Error("[meta_#%d] Attempt %d: Failed to write package.json for %s. Err: %v", workerID, attempt, pkg.Name, err)
			lastErr = err
			time.Sleep(time.Duration(attempt) * f.backoffFactor)
			continue
		}

		packages, err := f.remoteNpmRepo.DecodeNpmPackages(teeReader)
		teeReader.Close()
		if err != nil {
			f.logger.Error("[meta_#%d] Attempt %d: Failed to decode npm packages for %s. Err: %v", workerID, attempt, pkg.Name, err)
			lastErr = err
			time.Sleep(time.Duration(attempt) * f.backoffFactor)
			continue
		}

		return packages, nil
	}

	return nil, fmt.Errorf("failed to fetch metadata for %s. Err: %v", pkg.Name, lastErr)
}

// filterPackages filtre pre-release versions and versions that are less than the last version.
func (f *metadataWorkerPool) filterPackages(npmPackages []entities.NpmPackage, retrievePkg entities.RetrievePackage) []entities.NpmPackage {
	lastSyncDate := f.localNpmState.GetLastSync(retrievePkg)
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
