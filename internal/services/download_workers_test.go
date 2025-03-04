package services

import (
	"context"
	"errors"
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
		logger:             mockLogger,
		wg:                 &sync.WaitGroup{},
		localNpmRepo:       mockLocalRepo,
		remoteNpmRepo:      mockRemoteRepo,
		localNpmState:      mockLocalState,
		maxDownloadRetries: 1,
		backoffFactor:      time.Millisecond * 10,
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
		mockLogger.On("Debug", "[dl_#%d] Attempt %d: Downloading tarball for package %s:%s", workerID, 1, pkg.Name, pkg.Version.String()).Once()
		mockLogger.On("Debug", "[dl_#%d] Worker stopped due to inactivity", workerID)
		mockLogger.On("Error", "[dl_#%d] Attempt %d: Failed to download tarball for %s:%s. Err:%w", workerID, 1, pkg.Name, mock.Anything, mock.Anything).Once()
		mockLogger.On("Error", "[dl_#%d] Failed to download tarball for %s: %w", workerID, pkg.Name, mock.Anything).Once()
		mockRemoteRepo.On("DownloadTarballStream", mock.Anything, pkg.Url).Return(nil, assert.AnError).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		downloadChan := make(chan entities.NpmPackage, 10)
		downloadChan <- pkg

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

	const maxRetries = 5

	pool := &tarballWorkerPool{
		logger:             mockLogger,
		wg:                 &sync.WaitGroup{},
		localNpmRepo:       mockLocalRepo,
		remoteNpmRepo:      mockRemoteRepo,
		localNpmState:      mockLocalState,
		backoffFactor:      time.Millisecond * time.Duration(10),
		maxDownloadRetries: maxRetries,
	}

	t.Run("Erreur dans DownloadTarballStream", func(t *testing.T) {
		downloadErr := fmt.Errorf("download error")
		// Pour chaque tentative, on s'attend à ce que IsDebug retourne true, puis que le log de debug et l'appel à DownloadTarballStream soient faits
		mockLogger.On("IsDebug").Return(true).Times(maxRetries)
		for i := 1; i <= maxRetries; i++ {
			mockLogger.
				On("Debug", "[dl_#%d] Attempt %d: Downloading tarball for package %s:%s", workerID, i, packageName, dummyVersion.String()).
				Once()
			mockRemoteRepo.
				On("DownloadTarballStream", mock.Anything, dummyUrl).
				Return(nil, downloadErr).
				Once()
			mockLogger.
				On("Error", "[dl_#%d] Attempt %d: Failed to download tarball for %s:%s. Err:%w", workerID, i, packageName, dummyVersion.String(), downloadErr).
				Once()
		}

		ctx := context.Background()
		err := pool.downloadTarball(ctx, pkg, workerID)
		assert.Error(t, err)
		// Le message d'erreur final doit mentionner "after 5 attempts" et l'erreur retournée doit être celle de téléchargement
		assert.Contains(t, err.Error(), "after 5 attempts")
		assert.True(t, errors.Is(err, downloadErr))
	})

	t.Run("Erreur dans WriteTarball", func(t *testing.T) {
		dummyData := "tarball data"
		writeErr := fmt.Errorf("write tarball")
		// Pour chaque tentative, DownloadTarballStream réussit mais WriteTarball échoue
		mockLogger.On("IsDebug").Return(true).Times(maxRetries)
		for i := 1; i <= maxRetries; i++ {
			mockLogger.
				On("Debug", "[dl_#%d] Attempt %d: Downloading tarball for package %s:%s", workerID, i, packageName, dummyVersion.String()).
				Once()
			// À chaque tentative, retourner un nouveau reader
			reader := io.NopCloser(strings.NewReader(dummyData))
			mockRemoteRepo.
				On("DownloadTarballStream", mock.Anything, dummyUrl).
				Return(reader, nil).
				Once()
			mockLogger.
				On("Error", "[dl_#%d] Attempt %d: Failed to write tarball for %s:%s. Err:%w", workerID, i, packageName, dummyVersion.String(), writeErr).
				Once()
			mockLocalRepo.
				On("WriteTarball", packageName, dummyVersion.String(), mock.Anything, mock.Anything).
				Return(writeErr).
				Once()
		}

		ctx := context.Background()
		err := pool.downloadTarball(ctx, pkg, workerID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "after 5 attempts")
		assert.True(t, errors.Is(err, writeErr))
	})

	t.Run("Téléchargement réussi", func(t *testing.T) {
		dummyData := "tarball data"
		reader := io.NopCloser(strings.NewReader(dummyData))
		// Ici, seule la première tentative doit réussir.
		mockLogger.On("IsDebug").Return(true).Times(2)
		mockLogger.
			On("Debug", "[dl_#%d] Attempt %d: Downloading tarball for package %s:%s", workerID, 1, packageName, dummyVersion.String()).
			Once()
		mockRemoteRepo.
			On("DownloadTarballStream", mock.Anything, dummyUrl).
			Return(reader, nil).
			Once()
		mockLocalRepo.
			On("WriteTarball", packageName, dummyVersion.String(), mock.Anything, mock.Anything).
			Return(nil).
			Once()
		mockLocalState.On("IncrementDownloadedCount").Once()
		mockLogger.
			On("Debug", "[dl_#%d] Successfully downloaded tarball for package %s:%s", workerID, packageName, dummyVersion.String()).
			Once()

		ctx := context.Background()
		err := pool.downloadTarball(ctx, pkg, workerID)
		assert.NoError(t, err)
	})
}
