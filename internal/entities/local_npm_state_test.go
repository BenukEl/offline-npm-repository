package entities

import (
	"testing"

	"github.com/npmoffline/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetState_IsAnalysisNeeded(t *testing.T) {
	t.Run("Not existing package", func(t *testing.T) {
		ds := &localNpmState{
			states:          make(map[string]int),
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.True(t, ds.IsAnalysisNeeded("pkg1"))

		// A subsequent call should return the same state.
		assert.True(t, ds.IsAnalysisNeeded("pkg1"))
	})

	t.Run("Previously downloaded package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{"pkg1": PreviouslyInLocalRepoState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		assert.True(t, ds.IsAnalysisNeeded("pkg1"))
	})

	t.Run("Analysedd package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{"pkg1": AnalysedState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.False(t, ds.IsAnalysisNeeded("pkg1"))
	})
}

func TestGetState_IsAnalysisStarted(t *testing.T) {
	t.Run("Not existing package", func(t *testing.T) {
		ds := &localNpmState{
			states:          make(map[string]int),
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.False(t, ds.IsAnalysisStarted("pkg1"))

		// A subsequent call should return the same state.
		assert.False(t, ds.IsAnalysisStarted("pkg1"))
	})

	t.Run("Previously downloaded package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{"pkg1": PreviouslyInLocalRepoState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		assert.False(t, ds.IsAnalysisStarted("pkg1"))
	})

	t.Run("Analysed package", func(t *testing.T) {
		ds := &localNpmState{
			states:          map[string]int{"pkg1": AnalysedState},
			logger:          logger.NewMockLogger(t),
			downloadedCount: 0,
			analysedCount:   0,
		}

		// For an unknown package, GetState should return NotStartedState and initialize it.
		assert.True(t, ds.IsAnalysisStarted("pkg1"))
	})
}

func TestSetStateAndGetState(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	assert.False(t, ds.IsAnalysisStarted("pkg1"))

	// Update the state and version for "pkg1".
	ds.SetState("pkg1", AnalysingState)

	assert.True(t, ds.IsAnalysisStarted("pkg1"))

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
	ds.SetState("pkg1", AnalysingState)
	ds.SetState("pkg2", AnalysedState)

	packages = ds.GetPackages()
	assert.Len(t, packages, 2, "Two packages should be returned")
	assert.Contains(t, packages, RetrievePackage{Name: "pkg1"})
	assert.Contains(t, packages, RetrievePackage{Name: "pkg2"})
}
