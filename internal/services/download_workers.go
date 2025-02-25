package services

import (
	"context"
	"sync"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
)

// TarballWorkerPool defines the interface for tarball download workers.
type TarballWorkerPool interface {
	StartWorker(ctx context.Context, downloadChan chan entities.NpmPackage, workerID int, inactivityTime time.Duration)
	WaitAllWorkers()
}

// tarballWorkerPool is the concrete implementation of DownloadWorkerFactory.
type tarballWorkerPool struct {
	logger        logger.Logger
	wg            *sync.WaitGroup
	localNpmRepo  repositories.FileRepository
	remoteNpmRepo repositories.NpmRepository
	localNpmState entities.LocalNpmState
}

// NewTarballdWorkerPool creates a new instance of DownloadWorkerFactory.
func NewTarballdWorkerPool(logger logger.Logger, localNpmRepo repositories.FileRepository, remoteNpmRepo repositories.NpmRepository, localNpmState entities.LocalNpmState) TarballWorkerPool {
	return &tarballWorkerPool{
		logger:        logger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  localNpmRepo,
		remoteNpmRepo: remoteNpmRepo,
		localNpmState: localNpmState,
	}
}

// StartWorker launches a download worker.
func (f *tarballWorkerPool) StartWorker(ctx context.Context, downloadChan chan entities.NpmPackage, workerID int, inactivityTime time.Duration) {
	f.wg.Add(1)
	go func(id int) {
		defer f.wg.Done()

		f.logger.Debug("[dl_#%d] Worker started", id)

		// create a timer to stop the worker if it is inactive for a certain time.
		timer := time.NewTimer(inactivityTime)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				f.logger.Debug("[dl_#%d] Received context cancellation", id)
				return
			case pkg, ok := <-downloadChan:
				if !ok {
					f.logger.Debug("[dl_#%d] Download channel closed", id)
					return
				}
				if pkg.Name == "" {
					continue
				}
				if err := f.downloadTarball(ctx, pkg, id); err != nil {
					f.logger.Error("[dl_#%d] Failed to download tarball for %s: %w", id, pkg.Name, err)
				}
				// Reset the timer to avoid stopping the worker.
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(inactivityTime)
			case <-timer.C:
				f.logger.Debug("[dl_#%d] Worker stopped due to inactivity", id)
				return
			}
		}
	}(workerID)
}

// WaitAllWorkers waits for all workers to finish.
func (f *tarballWorkerPool) WaitAllWorkers() {
	f.wg.Wait()
}

// downloadTarball downloads the tarball for the given package.
func (f *tarballWorkerPool) downloadTarball(ctx context.Context, pkg entities.NpmPackage, workerID int) error {
	if f.logger.IsDebug() {
		f.logger.Debug("[dl_#%d] Downloading tarball for package %s:%s", workerID, pkg.Name, pkg.Version.String())
	}

	reader, err := f.remoteNpmRepo.DownloadTarballStream(ctx, pkg.Url)
	if err != nil {
		f.logger.Error("[dl_#%d] Failed to download tarball for %s:%s. Err:%w", workerID, pkg.Name, pkg.Version.String(), err)
		return err
	}
	defer reader.Close()

	if err := f.localNpmRepo.WriteTarball(pkg.Name, pkg.Version.String(), reader); err != nil {
		f.logger.Error("Failed to write tarball for %s: %w", pkg.Name, err)
		return err
	}

	f.localNpmState.IncrementDownloadedCount()
	if f.logger.IsDebug() {
		f.logger.Debug("[dl_#%d] Successfully downloaded tarball for package %s:%s", workerID, pkg.Name, pkg.Version.String())
	}

	return nil
}
