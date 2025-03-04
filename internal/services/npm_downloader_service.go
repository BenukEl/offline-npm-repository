package services

import (
	"context"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
)

const (
	channelBufferSize    = 100_000
	workerInactivityTime = 10 * time.Second
)

// DownloadPackagesOptions contains the options to start the download service.
type DownloadPackagesOptions struct {
	MetadataWorkers       int
	DownloadWorkers       int
	UpdateLocalRepository bool
}

// NpmDownloadService defines the interface of the download service.
type NpmDownloadService interface {
	DownloadPackages(ctx context.Context, packageList []string, options DownloadPackagesOptions)
}

type npmDownloadService struct {
	npmRepo            repositories.NpmRepository
	localNpmRepo       repositories.LocalNpmRepository
	logger             logger.Logger
	downloadState      entities.LocalNpmState
	startingDate       time.Time
	metadataWorkerPool MetadataWorkerPool
	tarballWorkerPool  TarballWorkerPool
}

// NewNpmDownloadService creates a new instance of the download service.
func NewNpmDownloadService(npmRepo repositories.NpmRepository, fileRepo repositories.LocalNpmRepository, log logger.Logger) *npmDownloadService {
	packages, lastSync, err := fileRepo.LoadDownloadedPackagesState()
	if err != nil {
		log.Error("Failed to load downloaded versions: %w", err)
		return nil
	}

	state := entities.NewLocalNpmState(packages, lastSync, log)

	return &npmDownloadService{
		npmRepo:            npmRepo,
		localNpmRepo:       fileRepo,
		logger:             log,
		downloadState:      state,
		startingDate:       time.Now().UTC(),
		metadataWorkerPool: NewMetadataWorkerPool(log, fileRepo, npmRepo, state),
		tarballWorkerPool:  NewTarballdWorkerPool(log, fileRepo, npmRepo, state),
	}
}

// DownloadPackages initiates the download process.
func (s *npmDownloadService) DownloadPackages(ctx context.Context, packageList []string, options DownloadPackagesOptions) {
	// Create local channels and waitgroups.
	metadataChan := make(chan entities.RetrievePackage, channelBufferSize)
	downloadChan := make(chan entities.NpmPackage, channelBufferSize)

	s.logger.Info("Starting download service")
	defer s.logger.Info("Download service stopped")

	// Start a ticker to log progress.
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				analysed := s.downloadState.GetAnalysedCount()
				downloaded := s.downloadState.GetDownloadedCount()
				s.logger.Info("Analysed: %d/%d, Downloaded: %d/%d", analysed, analysed+len(metadataChan), downloaded, downloaded+len(downloadChan))
			}
		}
	}()

	var retrievePackages []entities.RetrievePackage
	for _, pkg := range packageList {
		retrievePackages = append(retrievePackages, entities.NewRetrievePackage(pkg))
	}

	// Optionally update packageList with packages from the local state.
	if options.UpdateLocalRepository {
		s.logger.Info("Updating local repository: adding packages from download state to package list")
		packagesFromState := s.downloadState.GetPackages()
		retrievePackages = append(retrievePackages, packagesFromState...)
	}

	// Start metadata workers using the MetadataWorker pool.
	for i := 0; i < options.MetadataWorkers; i++ {
		s.metadataWorkerPool.StartWorker(ctx, metadataChan, downloadChan, i, workerInactivityTime)
	}

	// Start download workers using the TarballWorker pool.
	for i := 0; i < options.DownloadWorkers; i++ {
		s.tarballWorkerPool.StartWorker(ctx, downloadChan, i, workerInactivityTime)
	}

	// Enqueue the initial packages into the metadata channel.
	for _, pkg := range retrievePackages {
		metadataChan <- pkg
	}

	// Wait for metadata workers to finish.
	s.metadataWorkerPool.WaitAllWorkers()
	s.logger.Info("Metadata workers finished")
	close(metadataChan)

	// Wait for download workers to finish.
	s.tarballWorkerPool.WaitAllWorkers()
	s.logger.Info("Download workers finished")
	close(downloadChan)

	// Stop the ticker.
	ticker.Stop()

	// Save the download state.
	pkgs := s.downloadState.GetPackages()
	s.localNpmRepo.SaveDownloadedPackagesState(pkgs, s.startingDate)
}
