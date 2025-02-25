package entities

import (
	"sync"

	"github.com/npmoffline/internal/pkg/logger"
)

const (
	NotStartedState = iota
	AnalysingState
	AnalysedState
)

var ZeroVersion = SemVer{
	Major: 0,
	Minor: 0,
	Patch: 0,
}

// LocalNpmState defines the interface for the download state.
type LocalNpmState interface {
	GetState(packageName string) int
	SetState(packageName string, state int, version *SemVer)
	GetDownloadedCount() int
	IncrementDownloadedCount()
	GetAnalysedCount() int
	IncrementAnalysedCount()
	GetLastVersion(packageName string) SemVer
	GetVersions() map[string]SemVer
	GetPackages() []RetrievePackage
}

type localNpmState struct {
	mutex    sync.RWMutex
	states   map[string]int
	versions map[string]SemVer
	logger   logger.Logger

	downloadedCount int
	analysedCount   int
}

// NewLocalNpmState initialize a new download state from the given versions.
func NewLocalNpmState(fromVersions map[string]SemVer, logger logger.Logger) LocalNpmState {
	return &localNpmState{
		states:          make(map[string]int),
		versions:        fromVersions,
		logger:          logger,
		downloadedCount: 0,
		analysedCount:   0,
	}
}

// GetState returns the state of a package.
// If the package is not yet present, it is initialized with NotStartedState.
func (d *localNpmState) GetState(packageName string) int {
	d.mutex.RLock()
	state, ok := d.states[packageName]
	d.mutex.RUnlock()

	if !ok {
		d.mutex.Lock()
		// Check again in case another goroutine has already set the state
		if state, ok = d.states[packageName]; !ok {
			d.states[packageName] = NotStartedState
			state = NotStartedState
		}
		d.mutex.Unlock()
	}
	return state
}

// SetState updates the state of a package and its version.
func (d *localNpmState) SetState(packageName string, state int, version *SemVer) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.states[packageName] = state
	if version != nil && version.Compare(ZeroVersion) > 0 {
		d.versions[packageName] = *version
	}
}

// GetDownloadedCount return the number of downloaded packages.
func (d *localNpmState) GetDownloadedCount() int {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.downloadedCount
}

// IncrementDownloadedCount increments the downloaded packages count.
func (d *localNpmState) IncrementDownloadedCount() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.downloadedCount++
}

// GetAnalysedCount return the number of analysed packages.
func (d *localNpmState) GetAnalysedCount() int {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.analysedCount
}

// IncrementAnalysedCount increments the analysed packages count.
func (d *localNpmState) IncrementAnalysedCount() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.analysedCount++
}

// GetLastVersion returns the last version of a package.
func (d *localNpmState) GetLastVersion(packageName string) SemVer {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.versions[packageName]
}

// GetVersions returns a copy of the versions map.
func (d *localNpmState) GetVersions() map[string]SemVer {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	versionsCopy := make(map[string]SemVer, len(d.versions))
	for pkg, ver := range d.versions {
		versionsCopy[pkg] = ver
	}
	return versionsCopy
}

// GetPackages returns a slice of package names.
func (d *localNpmState) GetPackages() []RetrievePackage {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	packages := make([]RetrievePackage, 0, len(d.versions))
	for pkg := range d.versions {
		packages = append(packages, RetrievePackage{Name: pkg})
	}
	return packages
}
