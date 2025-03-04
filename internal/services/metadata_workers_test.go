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
	"github.com/stretchr/testify/require"
)

// for our tests, we use a low timeout value for worker inactivity.
var mockWorkerInactivityTime = 50 * time.Millisecond

func TestMetadataWorkerPool_StartWorker(t *testing.T) {
	mockLogger := logger.NewMockLogger(t)
	mockLocalRepo := repositories.NewMockLocalNpmRepository(t)
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
		mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything).Times(2)
		mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Times(1)

		mockLocalState.On("IsAnalysisNeeded", mock.Anything).Return(true).Once()

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
	mockLocalRepo := repositories.NewMockLocalNpmRepository(t)
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
	mockLocalRepo := repositories.NewMockLocalNpmRepository(t)
	mockRemoteRepo := repositories.NewMockNpmRepository(t)
	mockLocalState := entities.NewMockLocalNpmState(t)

	// Instanciation du pool avec quelques paramètres de configuration pour les tests.
	pool := &metadataWorkerPool{
		logger:             mockLogger,
		wg:                 &sync.WaitGroup{},
		localNpmRepo:       mockLocalRepo,
		remoteNpmRepo:      mockRemoteRepo,
		localNpmState:      mockLocalState,
		maxDownloadRetries: 1,
		backoffFactor:      time.Millisecond * 10,
	}

	t.Run("Skip processing if already processed", func(t *testing.T) {
		mockLocalState.On("IsAnalysisNeeded", testpkg).Return(false).Once()
		mockLogger.On("Debug", "Package %s already processed", packageName).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Exécution
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assertions
		assert.NoError(t, err)
		assert.Equal(t, 0, len(downloadChan))
		mockLogger.AssertCalled(t, "Debug", "Package %s already processed", packageName)
	})

	t.Run("Error in FetchMetadata", func(t *testing.T) {
		pool.maxDownloadRetries = 1
		mockLocalState.On("IsAnalysisNeeded", testpkg).Return(true).Once()
		// On s'attend à l'appel de fetchMetadata avec le message incluant "Attempt 1"
		mockLogger.On("IsDebug").Return(true).Once()
		mockLogger.On("Debug", "[meta_#%d] Attempt %d: Fetching metadata for package %s", workerID, 1, packageName).Once()

		fetchErr := fmt.Errorf("fetch error")
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(nil, fetchErr).Once()
		mockLogger.On("Error", "[meta_#%d] Attempt %d: Failed to fetch metadata for %s. Err: %w", workerID, 1, packageName, fetchErr).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Exécution
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch metadata for")
		mockLogger.AssertCalled(t, "Error", "[meta_#%d] Attempt %d: Failed to fetch metadata for %s. Err: %w", workerID, 1, packageName, fetchErr)
	})

	t.Run("Error in WritePackageJSON", func(t *testing.T) {
		pool.maxDownloadRetries = 1
		mockLocalState.On("IsAnalysisNeeded", testpkg).Return(true).Once()
		mockLogger.On("IsDebug").Return(true).Once()
		mockLogger.On("Debug", "[meta_#%d] Attempt %d: Fetching metadata for package %s", workerID, 1, packageName).Once()

		dummyData := "dummy metadata"
		reader := io.NopCloser(strings.NewReader(dummyData))
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(reader, nil).Once()

		writeErr := fmt.Errorf("write error")
		mockLocalRepo.On("WritePackageJSON", packageName, reader).Return(nil, writeErr).Once()
		mockLogger.On("Error", "[meta_#%d] Attempt %d: Failed to write package.json for %s. Err: %w", workerID, 1, packageName, writeErr).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Exécution
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch metadata for")
		mockLocalRepo.AssertCalled(t, "WritePackageJSON", packageName, reader)
	})

	t.Run("Error in DecodeNpmPackages", func(t *testing.T) {
		pool.maxDownloadRetries = 1
		mockLocalState.On("IsAnalysisNeeded", testpkg).Return(true).Once()
		mockLogger.On("IsDebug").Return(true).Once()
		mockLogger.On("Debug", "[meta_#%d] Attempt %d: Fetching metadata for package %s", workerID, 1, packageName).Once()

		dummyData := "dummy metadata"
		reader := io.NopCloser(strings.NewReader(dummyData))
		mockRemoteRepo.On("FetchMetadata", mock.Anything, packageName).Return(reader, nil).Once()

		teeReader := io.NopCloser(strings.NewReader(dummyData))
		mockLocalRepo.On("WritePackageJSON", packageName, reader).Return(teeReader, nil).Once()

		decodeErr := fmt.Errorf("decode error")
		mockRemoteRepo.On("DecodeNpmPackages", teeReader).Return(nil, decodeErr).Once()
		mockLogger.On("Error", "[meta_#%d] Attempt %d: Failed to decode npm packages for %s. Err: %w", workerID, 1, packageName, decodeErr).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Exécution
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch metadata for")
		mockRemoteRepo.AssertCalled(t, "DecodeNpmPackages", teeReader)
	})

	t.Run("Successful processing without dependencies", func(t *testing.T) {
		pool.maxDownloadRetries = 1
		mockLogger.On("IsDebug").Return(true).Times(2)
		mockLocalState.On("IsAnalysisNeeded", testpkg).Return(true).Once()
		mockLocalState.On("GetLastSync", testpkg).Return(time.Time{}).Once()

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
			ReleaseDate:  time.Now(),
		}
		mockRemoteRepo.On("DecodeNpmPackages", teeReader).Return([]entities.NpmPackage{pkg}, nil).Once()

		// On s'attend à l'appel de fetchMetadata avec le message d'Attempt 1
		mockLogger.On("Debug", "[meta_#%d] Attempt %d: Fetching metadata for package %s", workerID, 1, packageName).Once()
		mockLogger.On("Debug", "[meta_#%d] Enqueueing package %s:%s for download", workerID, pkg.Name, pkg.Version.String()).Once()
		mockLogger.On("Debug", "[meta_#%d] Processed package %s... %d versions to download", workerID, packageName, 1).Once()

		mockLocalState.On("SetState", testpkg, entities.AnalysedState).Once()
		mockLocalState.On("IncrementAnalysedCount").Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Exécution
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assertions
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
		pool.maxDownloadRetries = 1
		mockLogger.On("IsDebug").Return(true).Times(2)

		mockLocalState.On("IsAnalysisNeeded", testpkg).Return(true).Once()
		mockLocalState.On("GetLastSync", testpkg).Return(time.Time{}).Once()

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
			ReleaseDate:  time.Now(),
		}
		mockRemoteRepo.On("DecodeNpmPackages", teeReader).Return([]entities.NpmPackage{pkg}, nil).Once()

		mockLogger.On("Debug", "[meta_#%d] Attempt %d: Fetching metadata for package %s", workerID, 1, packageName).Once()
		mockLogger.On("Debug", "[meta_#%d] Enqueueing package %s:%s for download", workerID, pkg.Name, pkg.Version.String()).Once()
		mockLogger.On("Debug", "[meta_#%d] Processed package %s... %d versions to download", workerID, packageName, 1).Once()

		mockLocalState.On("SetState", testpkg, entities.AnalysedState).Once()
		mockLocalState.On("IncrementAnalysedCount").Once()

		dep1 := entities.NewRetrievePackage("dep1")
		peer1 := entities.NewRetrievePackage("peer1")
		mockLocalState.On("IsAnalysisStarted", dep1).Return(false).Once()
		mockLocalState.On("SetState", dep1, entities.AnalysingState).Once()
		mockLocalState.On("IsAnalysisStarted", peer1).Return(false).Once()
		mockLocalState.On("SetState", peer1, entities.AnalysingState).Once()

		ctx := context.Background()
		analyzeChan := make(chan entities.RetrievePackage, 10)
		downloadChan := make(chan entities.NpmPackage, 10)

		// Exécution
		err := pool.retrieveMetadata(ctx, testpkg, analyzeChan, downloadChan, workerID)

		// Assertions
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
		assert.Contains(t, deps, dep1)
		assert.Contains(t, deps, peer1)
	})
}

func TestMetadataWorkerPool_FilterPackages(t *testing.T) {
	// Création du mock pour localNpmState
	mockLocalState := entities.NewMockLocalNpmState(t)
	pool := &metadataWorkerPool{
		localNpmState: mockLocalState,
	}

	// Fonction d'aide pour créer un package avec une date de publication donnée
	toPkg := func(name, version string, releaseDate time.Time) entities.NpmPackage {
		ver, err := entities.NewSemVer(version)
		require.NoError(t, err)
		return entities.NpmPackage{
			Name:        name,
			Version:     ver,
			ReleaseDate: releaseDate,
		}
	}

	// Date de référence pour le dernier sync
	baseSync := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

	t.Run("Empty input returns empty slice", func(t *testing.T) {
		mockLocalState.On("GetLastSync", mock.Anything).Return(baseSync).Once()
		var pkgs []entities.NpmPackage

		filtered := pool.filterPackages(pkgs, entities.RetrievePackage{})
		assert.Empty(t, filtered, "Aucun package ne doit être retourné pour une entrée vide")
		mockLocalState.AssertExpectations(t)
	})

	t.Run("Excludes packages with release date not after last sync", func(t *testing.T) {
		// Pour le nom "pkg", on renvoie baseSync
		mockLocalState.On("GetLastSync", mock.Anything).Return(baseSync).Once()
		pkgs := []entities.NpmPackage{
			// Date égale à baseSync : non considérée comme postérieure
			toPkg("pkg", "1.0.0", baseSync),
			// Date avant baseSync
			toPkg("pkg", "1.0.1", baseSync.Add(-time.Hour)),
		}

		filtered := pool.filterPackages(pkgs, entities.RetrievePackage{Name: "pkg"})
		assert.Empty(t, filtered, "Aucun package ne doit être retourné si la date de publication n'est pas après le dernier sync")
		mockLocalState.AssertExpectations(t)
	})

	t.Run("Excludes non-matching pre-release versions", func(t *testing.T) {
		// Pour ce test, aucun pré-release ne doit être accepté
		mockLocalState.On("GetLastSync", mock.Anything).Return(baseSync).Once()
		// Ici, retrievePkg n'accepte pas (regex vide ou non-correspondant) les pré-release
		retrievePkg := entities.NewRetrievePackage("pkg")
		pkgs := []entities.NpmPackage{
			// Version pré-release "alpha" avec date postérieure
			toPkg("pkg", "1.0.1-alpha", baseSync.Add(time.Hour)),
		}

		filtered := pool.filterPackages(pkgs, retrievePkg)
		assert.Empty(t, filtered, "Les versions pré-release non correspondantes doivent être exclues")
		mockLocalState.AssertExpectations(t)
	})

	t.Run("Includes matching pre-release versions", func(t *testing.T) {
		// Ici, retrievePkg accepte les versions pré-release contenant "alpha"
		mockLocalState.On("GetLastSync", mock.Anything).Return(baseSync).Once()
		retrievePkg := entities.NewRetrievePackage("pkg|alpha")
		pkgs := []entities.NpmPackage{
			toPkg("pkg", "1.0.1-alpha", baseSync.Add(time.Hour)),
		}

		filtered := pool.filterPackages(pkgs, retrievePkg)
		assert.Len(t, filtered, 1, "La version pré-release correspondante doit être incluse")
		assert.Equal(t, "1.0.1-alpha", filtered[0].Version.String())
		mockLocalState.AssertExpectations(t)
	})

	t.Run("Includes non pre-release versions regardless of retrievePkg", func(t *testing.T) {
		// Les versions stables doivent être incluses si leur date de publication est après le dernier sync
		mockLocalState.On("GetLastSync", mock.Anything).Return(baseSync).Once()
		// Même si la regex de retrievePkg ne concerne que les pré-release, cela n'affecte pas les versions stables
		retrievePkg := entities.NewRetrievePackage("pkg|anything")
		pkgs := []entities.NpmPackage{
			toPkg("pkg", "1.2.0", baseSync.Add(2*time.Hour)),
		}

		filtered := pool.filterPackages(pkgs, retrievePkg)
		assert.Len(t, filtered, 1, "La version stable doit être incluse si la date de publication est après le dernier sync")
		assert.Equal(t, "1.2.0", filtered[0].Version.String())
		mockLocalState.AssertExpectations(t)
	})

	t.Run("Mixed packages filtering", func(t *testing.T) {
		// Test mixte avec plusieurs packages
		mockLocalState.On("GetLastSync", mock.Anything).Return(baseSync).Once()
		// Ici, retrievePkg n'accepte que les pré-release contenant "beta"
		retrievePkg := entities.NewRetrievePackage("pkg|beta")
		pkgs := []entities.NpmPackage{
			// Version stable postérieure
			toPkg("pkg", "1.0.0", baseSync.Add(2*time.Hour)),
			// Version pré-release "beta" (correspond) postérieure
			toPkg("pkg", "1.1.0-beta", baseSync.Add(3*time.Hour)),
			// Version pré-release "rc" (ne correspond pas) postérieure
			toPkg("pkg", "1.2.0-rc", baseSync.Add(4*time.Hour)),
			// Version stable antérieure : à exclure
			toPkg("pkg", "1.3.0", baseSync.Add(-time.Hour)),
			// Version stable avec date égale à baseSync : à exclure
			toPkg("pkg", "1.4.0", baseSync),
		}

		filtered := pool.filterPackages(pkgs, retrievePkg)
		// Seules "1.0.0" et "1.1.0-beta" doivent être retenues
		assert.Len(t, filtered, 2, "Seules les versions valides doivent être incluses")
		var versions []string
		for _, pkg := range filtered {
			versions = append(versions, pkg.Version.String())
		}
		assert.Contains(t, versions, "1.0.0")
		assert.Contains(t, versions, "1.1.0-beta")
		mockLocalState.AssertExpectations(t)
	})
}
