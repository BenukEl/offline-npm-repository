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

// for our tests, we use a low timeout value for worker inactivity.
var mockWorkerInactivityTime = 50 * time.Millisecond

func TestMetadataWorkerPool_StartWorker(t *testing.T) {
	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockFileRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	workerPool := &metadataWorkerPool{
		logger:        mockLogger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  mockLocalRepo,
		remoteNpmRepo: mockRemoteRepo,
		localNpmState: mockLocalState,
	}

	t.Run("Worker quit when context is cancelled", func(t *testing.T) {
		workerId := 1
		mockLogger.On("Debug", "[meta_#%d] Worker started", workerId).Once()
		mockLogger.On("Debug", "[meta_#%d] Received context cancellation", workerId).Once()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		workerPool.StartWorker(ctx, analyzeChan, downloadChan, workerId, mockWorkerInactivityTime)
		workerPool.WaitAllWorkers()

		assert.Equal(t, 0, len(downloadChan))
	})

	t.Run("Worker quit when analyzeChan is closed", func(t *testing.T) {
		workerId := 2
		mockLogger.On("Debug", "[meta_#%d] Worker started", workerId).Once()
		mockLogger.On("Debug", "[meta_#%d] Metadata channel closed", workerId).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		analyzeChan := make(chan entities.RetrievePackage, 10)
		close(analyzeChan)
		downloadChan := make(chan entities.NpmPackage, 10)

		workerPool.StartWorker(ctx, analyzeChan, downloadChan, workerId, mockWorkerInactivityTime)
		workerPool.WaitAllWorkers()

	})

	t.Run("Worker quit when inactivity duration is passed", func(t *testing.T) {
		workerId := 3
		mockLogger.On("Debug", "[meta_#%d] Worker started", workerId).Once()
		mockLogger.On("Debug", "[meta_#%d] Worker stopped due to inactivity", workerId).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		workerPool.StartWorker(ctx, analyzeChan, downloadChan, workerId, mockWorkerInactivityTime)
		workerPool.WaitAllWorkers()

	})

	t.Run("Skip package if its name is empty", func(t *testing.T) {
		workerId := 4
		mockLogger.On("Debug", mock.Anything, workerId).Times(2)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		analyzeChan := make(chan entities.RetrievePackage, 10)
		analyzeChan <- entities.RetrievePackage{Name: ""}
		downloadChan := make(chan entities.NpmPackage, 10)

		workerPool.StartWorker(ctx, analyzeChan, downloadChan, workerId, mockWorkerInactivityTime)
		workerPool.WaitAllWorkers()
	})

	t.Run("Retrieve metadata gives error", func(t *testing.T) {
		workerId := 5
		mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything).Times(3)
		mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Times(2)

		mockLocalState.On("GetState", mock.Anything).Return(entities.NotStartedState).Times(2)
		mockRemoteRepo.On("FetchMetadata", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		analyzeChan := make(chan entities.RetrievePackage, 10)
		analyzeChan <- entities.RetrievePackage{Name: "testpkg"}
		downloadChan := make(chan entities.NpmPackage, 10)

		workerPool.StartWorker(ctx, analyzeChan, downloadChan, workerId, mockWorkerInactivityTime)
		workerPool.WaitAllWorkers()
	})

}

func TestMetadataWorkerPool_WaitAllWorkers(t *testing.T) {
	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockFileRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	pool := &metadataWorkerPool{
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

		assert.Less(t, elapsed, 10*time.Millisecond, "WaitAllWorkers must return immediately if no worker")
	})

	t.Run("WaitAllWorkers waits for manual done", func(t *testing.T) {
		workerId := 1
		mockLogger.On("Debug", mock.Anything, workerId).Times(2)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		pool.StartWorker(ctx, analyzeChan, downloadChan, workerId, mockWorkerInactivityTime)

		start := time.Now()
		pool.WaitAllWorkers()
		elapsed := time.Since(start)

		assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "WaitAllWorkers must wait for workers to finish")
	})

}

func TestMetadataWorkerPool_RetrieveMetadata(t *testing.T) {
	packageName := "testpkg"
	testpkg := entities.RetrievePackage{Name: packageName}
	workerID := 1

	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockFileRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	pool := &metadataWorkerPool{
		logger:        mockLogger,
		wg:            &sync.WaitGroup{},
		localNpmRepo:  mockLocalRepo,
		remoteNpmRepo: mockRemoteRepo,
		localNpmState: mockLocalState,
	}

	t.Run("Skip processing if already processed", func(t *testing.T) {
		// Ack
		mockLocalState.On("GetState", packageName).Return(entities.AnalysedState).Times(2)
		mockLogger.On("Debug", "Package %s already processed", packageName).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Run
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 0, len(downloadChan))
		mockLogger.AssertCalled(t, "Debug", "Package %s already processed", packageName)
	})

	t.Run("Error in FetchMetadata", func(t *testing.T) {
		// Ack
		mockLocalState.On("GetState", packageName).Return(entities.NotStartedState).Times(2)
		mockLogger.On("Debug", "[meta_#%d] Fetching metadata for package: %s", workerID, packageName).Once()
		mockLogger.On("Error", "Failed to fetch metadata for %s: %w", packageName, mock.Anything).Once()

		fetchErr := fmt.Errorf("fetch error")
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(nil, fetchErr).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Run
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, fetchErr, err)
		mockLogger.AssertCalled(t, "Error", "Failed to fetch metadata for %s: %w", packageName, fetchErr)
	})

	t.Run("Error in WritePackageJSON", func(t *testing.T) {
		// Ack
		mockLocalState.On("GetState", packageName).Return(entities.NotStartedState).Times(2)
		mockLogger.On("Debug", "[meta_#%d] Fetching metadata for package: %s", workerID, packageName).Once()

		dummyData := "dummy metadata"
		reader := io.NopCloser(strings.NewReader(dummyData))
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(reader, nil).Once()

		writeErr := fmt.Errorf("write error")
		mockLocalRepo.On("WritePackageJSON", packageName, reader).Return(nil, writeErr).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Run
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write package.json for package")
		mockLocalRepo.AssertCalled(t, "WritePackageJSON", packageName, reader)
	})

	t.Run("Error in DecodeNpmPackages", func(t *testing.T) {
		// Ack
		mockLocalState.On("GetState", packageName).Return(entities.NotStartedState).Times(2)
		mockLogger.On("Debug", "[meta_#%d] Fetching metadata for package: %s", workerID, packageName).Once()

		dummyData := "dummy metadata"
		reader := io.NopCloser(strings.NewReader(dummyData))
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(reader, nil).Once()

		teeReader := io.NopCloser(strings.NewReader(dummyData))
		mockLocalRepo.On("WritePackageJSON", packageName, reader).Return(teeReader, nil).Once()

		decodeErr := fmt.Errorf("decode error")
		mockRemoteRepo.On("DecodeNpmPackages", teeReader).Return(nil, decodeErr).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Run
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode npm packages for package")
		mockRemoteRepo.AssertCalled(t, "DecodeNpmPackages", teeReader)
	})

	t.Run("Successful processing without dependencies", func(t *testing.T) {
		// Ack
		mockLogger.On("IsDebug").Return(true).Once()
		mockLocalState.On("GetState", packageName).Return(entities.NotStartedState).Times(2)
		initialVersion := entities.SemVer{Major: 0, Minor: 0, Patch: 0}
		mockLocalState.On("GetLastVersion", packageName).Return(initialVersion).Once()

		dummyData := "dummy metadata"
		reader := io.NopCloser(strings.NewReader(dummyData))
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(reader, nil).Once()

		teeReader := io.NopCloser(strings.NewReader(dummyData))
		mockLocalRepo.On("WritePackageJSON", packageName, reader).Return(teeReader, nil).Once()

		pkg := entities.NpmPackage{
			Name:         packageName,
			Version:      entities.SemVer{Major: 1, Minor: 0, Patch: 0},
			Dependencies: []string{},
			PeerDeps:     []string{},
		}
		mockRemoteRepo.On("DecodeNpmPackages", teeReader).Return([]entities.NpmPackage{pkg}, nil).Once()

		mockLogger.On("Debug", "[meta_#%d] Fetching metadata for package: %s", workerID, packageName).Once()
		mockLogger.On("Debug", "[meta_#%d] Enqueueing package %s:%s for download", workerID, pkg.Name, pkg.Version.String()).Once()
		mockLogger.On("Debug", "[meta_#%d] Processed package %s... %d versions to download (latest: %s)", workerID, packageName, 1, pkg.Version).Once()

		mockLocalState.On("SetState", packageName, entities.AnalysedState, &pkg.Version).Once()
		mockLocalState.On("IncrementAnalysedCount").Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Run
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assert
		assert.NoError(t, err)

		select {
		case receivedPkg := <-downloadChan:
			assert.Equal(t, pkg.Name, receivedPkg.Name)
			assert.Equal(t, pkg.Version, receivedPkg.Version)
		default:
			t.Error("The expected package was not enqueued in downloadChan")
		}

		assert.Equal(t, 0, len(analyzeChan))
	})

	t.Run("Successful processing with dependencies and peer dependencies", func(t *testing.T) {
		// Ack
		mockLogger.On("IsDebug").Return(true).Once()

		mockLocalState.On("GetState", mock.Anything).Return(entities.NotStartedState).Times(2)
		initialVersion := entities.SemVer{Major: 0, Minor: 0, Patch: 0}
		mockLocalState.On("GetLastVersion", packageName).Return(initialVersion).Once()

		dummyData := "dummy metadata"
		reader := io.NopCloser(strings.NewReader(dummyData))
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(reader, nil).Once()

		teeReader := io.NopCloser(strings.NewReader(dummyData))
		mockLocalRepo.On("WritePackageJSON", packageName, reader).Return(teeReader, nil).Once()

		pkg := entities.NpmPackage{
			Name:         packageName,
			Version:      entities.SemVer{Major: 2, Minor: 0, Patch: 0},
			Dependencies: []string{"dep1"},
			PeerDeps:     []string{"peer1"},
		}
		mockRemoteRepo.On("DecodeNpmPackages", teeReader).Return([]entities.NpmPackage{pkg}, nil).Once()

		mockLogger.On("Debug", "[meta_#%d] Fetching metadata for package: %s", workerID, packageName).Once()
		mockLogger.On("Debug", "[meta_#%d] Enqueueing package %s:%s for download", workerID, pkg.Name, pkg.Version.String()).Once()
		mockLogger.On("Debug", "[meta_#%d] Processed package %s... %d versions to download (latest: %s)", workerID, packageName, 1, pkg.Version).Once()

		mockLocalState.On("SetState", packageName, entities.AnalysedState, &pkg.Version).Once()
		mockLocalState.On("IncrementAnalysedCount").Once()

		mockLocalState.On("GetState", "dep1").Return(entities.NotStartedState).Once()
		mockLocalState.On("SetState", "dep1", entities.AnalysingState, (*entities.SemVer)(nil)).Once()
		mockLocalState.On("GetState", "peer1").Return(entities.NotStartedState).Once()
		mockLocalState.On("SetState", "peer1", entities.AnalysingState, (*entities.SemVer)(nil)).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Run
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assert
		assert.NoError(t, err)

		select {
		case receivedPkg := <-downloadChan:
			assert.Equal(t, pkg.Name, receivedPkg.Name)
			assert.Equal(t, pkg.Version, receivedPkg.Version)
		default:
			t.Error("Le package attendu n'a pas été enqueued dans downloadChan")
		}

		close(analyzeChan)
		var deps []entities.RetrievePackage
		for dep := range analyzeChan {
			deps = append(deps, dep)
		}
		assert.Contains(t, deps, entities.RetrievePackage{Name: "dep1"})
		assert.Contains(t, deps, entities.RetrievePackage{Name: "peer1"})
	})
}

func TestMetadataWorkerPool_FilterPackages(t *testing.T) {
	mockLocalState := entities.NewMockLocalNpmState(t)

	pool := &metadataWorkerPool{
		localNpmState: mockLocalState,
	}

	toPkg := func(name string, version string) entities.NpmPackage {
		ver, _ := entities.NewSemVer(version)

		return entities.NpmPackage{
			Name:    name,
			Version: ver,
		}
	}

	t.Run("Empty input returns empty slice", func(t *testing.T) {
		lastVersion := entities.SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: ""}
		mockLocalState.On("GetLastVersion", mock.Anything).Return(lastVersion).Once()

		var packages []entities.NpmPackage

		filtered := pool.filterPackages(packages, entities.RetrievePackage{})
		assert.Empty(t, filtered, "Aucun package ne doit être retourné pour une entrée vide")
	})

	t.Run("Excludes pre-release and packages not greater than lastVersion", func(t *testing.T) {
		lastVersion := entities.SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: ""}
		mockLocalState.On("GetLastVersion", mock.Anything).Return(lastVersion).Once()

		packages := []entities.NpmPackage{
			toPkg("pkg", "0.9.9"),
			toPkg("pkg", "1.0.0"),
			toPkg("pkg", "1.1.0"),
			toPkg("pkg", "2.0.0-beta"),
			toPkg("pkg", "1.2.0"),
		}

		filtered := pool.filterPackages(packages, entities.RetrievePackage{})

		assert.Len(t, filtered, 2, "Deux packages devraient être retournés")
		versions := []string{filtered[0].Version.String(), filtered[1].Version.String()}
		assert.Contains(t, versions, "1.1.0", "The verison 1.1.0 should be included")
		assert.Contains(t, versions, "1.2.0", "The verison 1.2.0 should be included")
	})

	t.Run("Includeds pre-release matching with RetievePackage regex", func(t *testing.T) {
		lastVersion := entities.SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: ""}
		mockLocalState.On("GetLastVersion", mock.Anything).Return(lastVersion).Once()

		packages := []entities.NpmPackage{
			toPkg("pkg", "0.9.9"),
			toPkg("pkg", "1.0.1-0"),
			toPkg("pkg", "1.1.0-rc"),
			toPkg("pkg", "2.0.0"),
			toPkg("pkg", "1.2.0-3"),
		}

		filtered := pool.filterPackages(packages, entities.NewRetrievePackage("pkg|\\d"))

		assert.Len(t, filtered, 3, "Two packages should be returned")
		var versions []string
		for _, pkg := range filtered {
			versions = append(versions, pkg.Version.String())
		}
		assert.Contains(t, versions, "1.0.1-0", "The verison 1.0.1-0 should be included")
		assert.Contains(t, versions, "2.0.0", "The verison 2.0.0 should be included")
		assert.Contains(t, versions, "1.2.0-3", "The verison 1.2.0-3 should be included")
	})
}
