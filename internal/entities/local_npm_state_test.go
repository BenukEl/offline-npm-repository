package entities

import (
	"testing"

	"github.com/npmoffline/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetState_NotExisting(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		versions:        make(map[string]SemVer),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	// For an unknown package, GetState should return NotStartedState and initialize it.
	state := ds.GetState("pkg1")
	assert.Equal(t, NotStartedState, state, "Initial state should be NotStartedState")

	// A subsequent call should return the same state.
	state2 := ds.GetState("pkg1")
	assert.Equal(t, NotStartedState, state2, "State should remain NotStartedState")
}

func TestSetStateAndGetLastVersion(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		versions:        make(map[string]SemVer),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}
	version := SemVer{Major: 1, Minor: 0, Patch: 0}

	// Update the state and version for "pkg1".
	ds.SetState("pkg1", AnalysingState, &version)

	state := ds.GetState("pkg1")
	assert.Equal(t, AnalysingState, state, "State should be AnalysingState")

	lastVersion := ds.GetLastVersion("pkg1")
	assert.Equal(t, version, lastVersion, "Returned version should match the set version")
}

func TestIncrementDownloadedCount(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		versions:        make(map[string]SemVer),
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
		versions:        make(map[string]SemVer),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	initial := ds.GetAnalysedCount()
	ds.IncrementAnalysedCount()
	count := ds.GetAnalysedCount()
	assert.Equal(t, initial+1, count, "Analysed count should be incremented")
}

func TestGetVersions(t *testing.T) {
	ds := &localNpmState{
		states: map[string]int{},
		versions: map[string]SemVer{
			"pkg1": {Major: 1, Minor: 0, Patch: 0},
		},
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	// Update the version for pkg1.
	newVersion := SemVer{Major: 2, Minor: 0, Patch: 0}
	ds.SetState("pkg1", AnalysingState, &newVersion)

	versions := ds.GetVersions()
	v, ok := versions["pkg1"]
	assert.True(t, ok, "The versions map should contain pkg1")
	assert.Equal(t, newVersion, v, "The version for pkg1 should be updated")

	// Verify that the returned copy is independent of the internal state.
	versions["pkg1"] = SemVer{Major: 999, Minor: 0, Patch: 0}
	internalVersion := ds.GetLastVersion("pkg1")
	assert.NotEqual(t, 999, internalVersion.Major, "Modifying the returned copy should not affect the internal state")
}

func TestGetPackages(t *testing.T) {
	ds := &localNpmState{
		states:          make(map[string]int),
		versions:        make(map[string]SemVer),
		logger:          logger.NewMockLogger(t),
		downloadedCount: 0,
		analysedCount:   0,
	}

	// Initially, no version is defined: GetPackages should return an empty slice.
	packages := ds.GetPackages()
	assert.Empty(t, packages, "No packages should be returned initially")

	// Add two packages with versions.
	version1 := SemVer{Major: 1, Minor: 0, Patch: 0}
	version2 := SemVer{Major: 2, Minor: 0, Patch: 0}
	ds.SetState("pkg1", AnalysingState, &version1)
	ds.SetState("pkg2", AnalysedState, &version2)

	packages = ds.GetPackages()
	assert.Len(t, packages, 2, "Two packages should be returned")
	assert.Contains(t, packages, RetrievePackage{Name: "pkg1"})
	assert.Contains(t, packages, RetrievePackage{Name: "pkg2"})
}
