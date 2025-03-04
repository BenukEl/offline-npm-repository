package entities

import (
	"sync"
	"time"

	"github.com/npmoffline/internal/pkg/logger"
)

const (
	PreviouslyInLocalRepoState = iota
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
	GetDownloadedCount() int
	IncrementDownloadedCount()
	GetAnalysedCount() int
	IncrementAnalysedCount()
	GetLastSync(pkg RetrievePackage) time.Time
	GetPackages() []RetrievePackage
	SetState(pkg RetrievePackage, state int)
	IsAnalysisNeeded(pkg RetrievePackage) bool
	IsAnalysisStarted(pkg RetrievePackage) bool
}

type localNpmState struct {
	mutex    sync.RWMutex
	states   map[string]int
	logger   logger.Logger
	lastSync time.Time

	downloadedCount int
	analysedCount   int
}

// NewLocalNpmState initialize a new download state from the given versions.
func NewLocalNpmState(packages []RetrievePackage, lastSync time.Time, logger logger.Logger) LocalNpmState {
	states := make(map[string]int, len(packages))
	for _, pkg := range packages {
		states[pkg.String()] = PreviouslyInLocalRepoState
	}

	return &localNpmState{
		states:          states,
		lastSync:        lastSync,
		logger:          logger,
		downloadedCount: 0,
		analysedCount:   0,
	}
}

// SetState updates the state of a package and its version.
func (d *localNpmState) SetState(pkg RetrievePackage, state int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.states[pkg.String()] = state
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

// GetLastSync returns the last sync date.
func (d *localNpmState) GetLastSync(pkg RetrievePackage) time.Time {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if _, ok := d.states[pkg.String()]; !ok {
		return time.Time{}
	}
	return d.lastSync
}

// GetPackages returns a slice of package names.
func (d *localNpmState) GetPackages() []RetrievePackage {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	packages := make([]RetrievePackage, 0, len(d.states))
	for pkg := range d.states {
		packages = append(packages, NewRetrievePackage(pkg))
	}
	return packages
}

// IsAnalysisNeeded returns true if the package needs to be analysed.
func (d *localNpmState) IsAnalysisNeeded(pkg RetrievePackage) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	state, ok := d.states[pkg.String()]
	return !ok || state != AnalysedState
}

// IsAnalysisStarted returns true if the package analysis has started.
func (d *localNpmState) IsAnalysisStarted(pkg RetrievePackage) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	state, ok := d.states[pkg.String()]
	return ok && state != PreviouslyInLocalRepoState
}
