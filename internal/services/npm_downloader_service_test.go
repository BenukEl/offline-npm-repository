package services

import (
	"context"
	"testing"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNpmDownloadService_DownloadPackages(t *testing.T) {
	// Create mocks for dependencies.
	mockNpmRepo := repositories.NewMockNpmRepository(t)
	mockLocalNpmRepo := repositories.NewMockLocalNpmRepository(t)
	mockLogger := logger.NewMockLogger(t)
	mockLocalState := entities.NewMockLocalNpmState(t)
	mockMetadataPool := NewMockMetadataWorkerPool(t)
	mockTarballPool := NewMockTarballWorkerPool(t)

	service := &npmDownloadService{
		npmRepo:            mockNpmRepo,
		localNpmRepo:       mockLocalNpmRepo,
		logger:             mockLogger,
		downloadState:      mockLocalState,
		metadataWorkerPool: mockMetadataPool,
		tarballWorkerPool:  mockTarballPool,
		startingDate:       time.Now().UTC(),
	}

	t.Run("Download packages with cancelled context", func(t *testing.T) {
		mockLocalState.On("GetPackages").Return([]entities.RetrievePackage{}).Once()
		mockLocalNpmRepo.On("SaveDownloadedPackagesState", mock.Anything, mock.Anything).Return(nil).Once()

		mockLogger.On("Info", mock.Anything).Return().Times(4)

		mockMetadataPool.On("WaitAllWorkers").Return().Once()
		mockTarballPool.On("WaitAllWorkers").Return().Once()

		// Create a context that is canceled immediately.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Run Start in a separate goroutine since it is blocking.
		go service.DownloadPackages(ctx, []string{"pkg1"}, DownloadPackagesOptions{
			MetadataWorkers:       0,
			DownloadWorkers:       0,
			UpdateLocalRepository: false,
		})

		// Give a short time for the service to exit.
		time.Sleep(100 * time.Millisecond)

		// Verify that SaveDownloadedPackagesState was called.
		mockLocalNpmRepo.AssertCalled(t, "SaveDownloadedPackagesState", mock.Anything, mock.Anything)

	})

	t.Run("Download packages with update local repository", func(t *testing.T) {

		expectedStatePkgs := []entities.RetrievePackage{{Name: "statePkg1"}, {Name: "statePkg2"}}
		mockLocalState.On("GetPackages").Return(expectedStatePkgs).Times(2)

		mockLocalNpmRepo.On("SaveDownloadedPackagesState", mock.Anything, mock.Anything).Return(nil).Once()

		mockLogger.On("Info", mock.Anything).Return().Times(5)
		var packageChannel chan entities.RetrievePackage
		mockMetadataPool.On("StartWorker", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				packageChannel = args.Get(1).(chan entities.RetrievePackage)
			}).Return().Once()

		mockMetadataPool.On("WaitAllWorkers").Return().Once()
		mockTarballPool.On("WaitAllWorkers").Return().Once()

		// Use a context with timeout to allow the service to complete.
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Call Start with UpdateLocalRepository true.
		initialPkgs := []string{"initialPkg"}
		service.DownloadPackages(ctx, initialPkgs, DownloadPackagesOptions{
			MetadataWorkers:       1,
			DownloadWorkers:       0,
			UpdateLocalRepository: true,
		})

		assert.Equal(t, 3, len(packageChannel))
		// Assert that GetPackages was called on local state.
		mockLocalState.AssertCalled(t, "GetPackages")
		// Assert that SaveDownloadedPackagesState was called.
		mockLocalNpmRepo.AssertCalled(t, "SaveDownloadedPackagesState", mock.Anything, mock.Anything)
	})

	t.Run("starts same worker number as option say", func(t *testing.T) {
		mockLocalState.On("GetPackages").Return([]entities.RetrievePackage{}).Once()
		mockLocalNpmRepo.On("SaveDownloadedPackagesState", mock.Anything, mock.Anything).Return(nil).Once()

		mockLogger.On("Info", mock.Anything).Return().Times(4)

		mockMetadataPool.On("StartWorker", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Times(6)
		mockTarballPool.On("StartWorker", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Times(4)

		mockMetadataPool.On("WaitAllWorkers").Return().Once()
		mockTarballPool.On("WaitAllWorkers").Return().Once()

		// Use a context with timeout to allow the service to complete.
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Call Start with UpdateLocalRepository true.
		initialPkgs := []string{"initialPkg"}
		service.DownloadPackages(ctx, initialPkgs, DownloadPackagesOptions{
			MetadataWorkers:       6,
			DownloadWorkers:       4,
			UpdateLocalRepository: false,
		})

		// Assert that SaveDownloadedPackagesState was called.
		mockLocalNpmRepo.AssertCalled(t, "SaveDownloadedPackagesState", mock.Anything, mock.Anything)

	})
}
