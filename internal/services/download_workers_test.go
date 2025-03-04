package services

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTarballWorkerPool_StartWorker(t *testing.T) {
	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockLocalNpmRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	pool := &tarballWorkerPool{
		logger:        mockLogger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  mockLocalRepo,
		remoteNpmRepo: mockRemoteRepo,
		localNpmState: mockLocalState,
	}

	t.Run("Worker terminates when context is cancelled", func(t *testing.T) {

		workerID := 1
		mockLogger.On("Debug", "[dl_#%d] Worker started", workerID).Once()
		mockLogger.On("Debug", "[dl_#%d] Received context cancellation", workerID).Once()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		pool.StartWorker(ctx, downloadChan, workerID, mockWorkerInactivityTime)
		pool.WaitAllWorkers()
	})

	t.Run("Worker terminates when download channel is closed", func(t *testing.T) {

		workerID := 2
		mockLogger.On("Debug", "[dl_#%d] Worker started", workerID).Once()
		mockLogger.On("Debug", "[dl_#%d] Download channel closed", workerID).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		close(downloadChan)

		pool.StartWorker(ctx, downloadChan, workerID, mockWorkerInactivityTime)
		pool.WaitAllWorkers()
	})

	t.Run("Worker stops due to inactivity", func(t *testing.T) {

		workerID := 3
		mockLogger.On("Debug", "[dl_#%d] Worker started", workerID).Once()
		mockLogger.On("Debug", "[dl_#%d] Worker stopped due to inactivity", workerID).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		pool.StartWorker(ctx, downloadChan, workerID, mockWorkerInactivityTime)
		pool.WaitAllWorkers()
	})

	t.Run("Ignore package if its name is empty", func(t *testing.T) {

		workerID := 4
		mockLogger.On("Debug", "[dl_#%d] Worker started", workerID).Once()
		mockLogger.On("Debug", "[dl_#%d] Worker stopped due to inactivity", workerID).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		downloadChan <- entities.NpmPackage{Name: ""}

		pool.StartWorker(ctx, downloadChan, workerID, mockWorkerInactivityTime)
		pool.WaitAllWorkers()
	})

	t.Run("Worker logs error when download fails", func(t *testing.T) {

		workerID := 5
		pkg := entities.NpmPackage{
			Name:    "pkg1",
			Url:     "http://example.com/pkg1",
			Version: entities.SemVer{Major: 1, Minor: 0, Patch: 0},
		}

		mockLogger.On("IsDebug").Return(true).Once()
		mockLogger.On("Debug", "[dl_#%d] Worker started", workerID).Once()
		mockLogger.On("Debug", "[dl_#%d] Downloading tarball for package %s:%s", workerID, pkg.Name, pkg.Version.String()).Once()
		mockLogger.On("Debug", "[dl_#%d] Worker stopped due to inactivity", workerID)
		downloadErr := fmt.Errorf("download error")
		mockLogger.On("Error", "[dl_#%d] Failed to download tarball for %s:%s. Err:%w", workerID, pkg.Name, pkg.Version.String(), downloadErr).Once()
		mockLogger.On("Error", "[dl_#%d] Failed to download tarball for %s: %w", workerID, pkg.Name, downloadErr).Once()
		mockRemoteRepo.On("DownloadTarballStream", mock.Anything, pkg.Url).Return(nil, downloadErr).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		downloadChan <- pkg

		pool.StartWorker(ctx, downloadChan, workerID, mockWorkerInactivityTime)
		pool.WaitAllWorkers()
	})
}

func TestTarballWorkerPool_WaitAllWorkers(t *testing.T) {
	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockLocalNpmRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	pool := &tarballWorkerPool{
		logger:        mockLogger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  mockLocalRepo,
		remoteNpmRepo: mockRemoteRepo,
		localNpmState: mockLocalState,
	}

	t.Run("WaitAllWorkers returns immediately if no worker", func(t *testing.T) {
		start := time.Now()
		pool.WaitAllWorkers()
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 10*time.Millisecond, "WaitAllWorkers should return immediately if no worker")
	})

	t.Run("WaitAllWorkers waits for workers to finish", func(t *testing.T) {
		workerID := 1
		mockLogger.On("Debug", "[dl_#%d] Worker started", workerID).Once()
		mockLogger.On("Debug", "[dl_#%d] Worker stopped due to inactivity", workerID).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		pool.StartWorker(ctx, downloadChan, workerID, mockWorkerInactivityTime)
		pool.WaitAllWorkers()
	})
}

func TestTarballWorkerPool_downloadTarball(t *testing.T) {
	packageName := "testpkg"
	workerID := 1
	dummyUrl := "http://example.com/tarball"
	dummyVersion := entities.SemVer{Major: 1, Minor: 0, Patch: 0}
	pkg := entities.NpmPackage{
		Name:    packageName,
		Url:     dummyUrl,
		Version: dummyVersion,
	}

	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockLocalNpmRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	pool := &tarballWorkerPool{
		logger:        mockLogger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  mockLocalRepo,
		remoteNpmRepo: mockRemoteRepo,
		localNpmState: mockLocalState,
	}

	t.Run("Error in DownloadTarballStream", func(t *testing.T) {
		downloadErr := fmt.Errorf("download error")
		mockLogger.On("IsDebug").Return(true).Once()
		mockLogger.On("Debug", "[dl_#%d] Downloading tarball for package %s:%s", workerID, packageName, dummyVersion.String()).Once()
		mockRemoteRepo.On("DownloadTarballStream", mock.Anything, dummyUrl).Return(nil, downloadErr).Once()
		mockLogger.On("Error", "[dl_#%d] Failed to download tarball for %s:%s. Err:%w", workerID, packageName, dummyVersion.String(), downloadErr).Once()

		ctx := context.Background()
		err := pool.downloadTarball(ctx, pkg, workerID)
		assert.Error(t, err)
		assert.Equal(t, downloadErr, err)
	})

	t.Run("Error in WriteTarball", func(t *testing.T) {
		dummyData := "tarball data"
		reader := io.NopCloser(strings.NewReader(dummyData))

		mockLogger.On("IsDebug").Return(true).Once()
		mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()
		mockRemoteRepo.On("DownloadTarballStream", mock.Anything, dummyUrl).Return(reader, nil).Once()

		writeErr := fmt.Errorf("write tarball")
		mockLogger.On("Error", "Failed to write tarball for %s: %w", packageName, writeErr).Once()
		mockLocalRepo.On("WriteTarball", packageName, dummyVersion.String(), mock.Anything).Return(writeErr).Once()

		ctx := context.Background()
		err := pool.downloadTarball(ctx, pkg, workerID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "write tarball")
	})

	t.Run("Successful download", func(t *testing.T) {
		dummyData := "tarball data"
		reader := io.NopCloser(strings.NewReader(dummyData))

		mockLogger.On("IsDebug").Return(true).Times(2)
		mockLogger.On("Debug", "[dl_#%d] Downloading tarball for package %s:%s", workerID, packageName, dummyVersion.String()).Once()
		mockLogger.On("Debug", "[dl_#%d] Successfully downloaded tarball for package %s:%s", workerID, packageName, dummyVersion.String()).Once()

		mockRemoteRepo.On("DownloadTarballStream", mock.Anything, dummyUrl).Return(reader, nil).Once()
		mockLocalRepo.On("WriteTarball", packageName, dummyVersion.String(), mock.Anything).Return(nil).Once()
		mockLocalState.On("IncrementDownloadedCount").Once()

		ctx := context.Background()
		err := pool.downloadTarball(ctx, pkg, workerID)
		assert.NoError(t, err)
	})
}
