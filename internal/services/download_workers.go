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

const maxRetries = 5

// TarballWorkerPool defines the interface for tarball download workers.
type TarballWorkerPool interface {
	StartWorker(ctx context.Context, downloadChan <-chan entities.NpmPackage, workerID int, inactivityTime time.Duration)
	WaitAllWorkers()
}

// tarballWorkerPool is the concrete implementation of DownloadWorkerFactory.
type tarballWorkerPool struct {
	logger             logger.Logger
	wg                 *sync.WaitGroup
	localNpmRepo       repositories.LocalNpmRepository
	remoteNpmRepo      repositories.NpmRepository
	localNpmState      entities.LocalNpmState
	backoffFactor      time.Duration
	maxDownloadRetries int
}

// NewTarballdWorkerPool creates a new instance of DownloadWorkerFactory.
func NewTarballdWorkerPool(logger logger.Logger, localNpmRepo repositories.LocalNpmRepository, remoteNpmRepo repositories.NpmRepository, localNpmState entities.LocalNpmState) TarballWorkerPool {
	return &tarballWorkerPool{
		logger:             logger,
		wg:                 &sync.WaitGroup{},
		localNpmRepo:       localNpmRepo,
		remoteNpmRepo:      remoteNpmRepo,
		localNpmState:      localNpmState,
		backoffFactor:      time.Second,
		maxDownloadRetries: maxRetries,
	}
}

// StartWorker launches a download worker.
func (p *tarballWorkerPool) StartWorker(ctx context.Context, downloadChan <-chan entities.NpmPackage, workerID int, inactivityTime time.Duration) {
	p.wg.Add(1)
	go func(id int) {
		defer p.wg.Done()

		p.logger.Debug("[dl_#%d] Worker started", id)

		// create a timer to stop the worker if it is inactive for a certain time.
		timer := time.NewTimer(inactivityTime)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.Debug("[dl_#%d] Received context cancellation", id)
				return
			case pkg, ok := <-downloadChan:
				if !ok {
					p.logger.Debug("[dl_#%d] Download channel closed", id)
					return
				}
				if pkg.Name == "" {
					continue
				}
				if err := p.downloadTarball(ctx, pkg, id); err != nil {
					p.logger.Error("[dl_#%d] Failed to download tarball for %s: %w", id, pkg.Name, err)
				}
				// Reset the timer to avoid stopping the worker.
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(inactivityTime)
			case <-timer.C:
				p.logger.Debug("[dl_#%d] Worker stopped due to inactivity", id)
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
func (p *tarballWorkerPool) downloadTarball(ctx context.Context, pkg entities.NpmPackage, workerID int) error {
	var lastErr error

	for attempt := 1; attempt <= p.maxDownloadRetries; attempt++ {
		if p.logger.IsDebug() {
			p.logger.Debug("[dl_#%d] Attempt %d: Downloading tarball for package %s:%s", workerID, attempt, pkg.Name, pkg.Version.String())
		}

		reader, err := p.remoteNpmRepo.DownloadTarballStream(ctx, pkg.Url)
		if err != nil {
			p.logger.Error("[dl_#%d] Attempt %d: Failed to download tarball for %s:%s. Err:%w", workerID, attempt, pkg.Name, pkg.Version.String(), err)
			lastErr = err
			// Délai progressif avant la prochaine tentative
			time.Sleep(time.Duration(attempt) * p.backoffFactor)
			continue
		}

		err = p.localNpmRepo.WriteTarball(pkg.Name, pkg.Version.String(), pkg.Integrity, reader)
		reader.Close()
		if err != nil {
			p.logger.Error("[dl_#%d] Attempt %d: Failed to write tarball for %s:%s. Err:%w", workerID, attempt, pkg.Name, pkg.Version.String(), err)
			lastErr = err
			time.Sleep(time.Duration(attempt) * p.backoffFactor)
			continue
		}

		p.localNpmState.IncrementDownloadedCount()
		if p.logger.IsDebug() {
			p.logger.Debug("[dl_#%d] Successfully downloaded tarball for package %s:%s", workerID, pkg.Name, pkg.Version.String())
		}
		return nil
	}

	return fmt.Errorf("failed to download tarball for package %s after %d attempts: %w", pkg.Name, p.maxDownloadRetries, lastErr)
}
