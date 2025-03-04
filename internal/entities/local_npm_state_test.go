package entities

import (
	"testing"

	"github.com/npmoffline/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetState_IsAnalysisNeeded(t *testing.T) {
	pkg := NewRetrievePackage("pkg1|alpha")
	t.Run("Not existing package", func(t *testing.T) {
		ds := &localNpmState{
			states:          make(map[string]int),
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.True(t, ds.IsAnalysisNeeded(pkg))

		// A subsequent call should return the same state.
		assert.True(t, ds.IsAnalysisNeeded(pkg))
	})

	t.Run("Previously downloaded package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{pkg.String(): PreviouslyInLocalRepoState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		assert.True(t, ds.IsAnalysisNeeded(pkg))
	})

	t.Run("Analysedd package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{pkg.String(): AnalysedState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.False(t, ds.IsAnalysisNeeded(pkg))
	})
}

func TestGetState_IsAnalysisStarted(t *testing.T) {
	pkg := NewRetrievePackage("pkg1|toto")
	t.Run("Not existing package", func(t *testing.T) {
		ds := &localNpmState{
			states:          make(map[string]int),
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.False(t, ds.IsAnalysisStarted(pkg))

		// A subsequent call should return the same state.
		assert.False(t, ds.IsAnalysisStarted(pkg))
	})

	t.Run("Previously downloaded package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{pkg.String(): PreviouslyInLocalRepoState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		assert.False(t, ds.IsAnalysisStarted(pkg))
	})

	t.Run("Analysed package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{pkg.String(): AnalysedState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.True(t, ds.IsAnalysisStarted(pkg))
	})
}

func TestSetStateAndGetState(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}
	pkg := NewRetrievePackage("pkg1|toto")
	assert.False(t, ds.IsAnalysisStarted(pkg))

	// Update the state and version for "pkg1".
	ds.SetState(pkg, AnalysingState)

	assert.True(t, ds.IsAnalysisStarted(pkg))

}

func TestIncrementDownloadedCount(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	initial := ds.GetDownloadedCount()
	ds.IncrementDownloadedCount()
	count := ds.GetDownloadedCount()
	assert.Equal(t, initial+1, count, "Downloaded count should be incremented")
}

func TestIncrementAnalysedCount(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	initial := ds.GetAnalysedCount()
	ds.IncrementAnalysedCount()
	count := ds.GetAnalysedCount()
	assert.Equal(t, initial+1, count, "Analysed count should be incremented")
}

func TestGetPackages(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	// Initially, no version is defined: GetPackages should return an empty slice.
	packages := ds.GetPackages()
	assert.Empty(t, packages, "No packages should be returned initially")

	// Add two packages with versions.
	ds.SetState(NewRetrievePackage("pkg1"), AnalysingState)
	ds.SetState(NewRetrievePackage("pkg2|"), AnalysedState)

	packages = ds.GetPackages()
	assert.Len(t, packages, 2, "Two packages should be returned")
	assert.Contains(t, packages, RetrievePackage{Name: "pkg1", fullName: "pkg1", allowedPreVersion: nil})
	assert.Contains(t, packages, RetrievePackage{Name: "pkg2", fullName: "pkg2", allowedPreVersion: nil})
}
