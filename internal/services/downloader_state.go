package services

import (
	"sync"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
)

const (
	NotStartedState = iota
	AnalysingState
	AnalysedState
)

var ZERO_VERSION = entities.SemVer{
	Major: 0,
	Minor: 0,
	Patch: 0,
}

type DownloadState interface {
	GetState(packageName string) int
	SetState(packageName string, state int, version *entities.SemVer)
	GetDownloadedCount() int
	IncrementDownloadedCount()
	GetAnalysedCount() int
	IncrementAnalysedCount()
	GetLastVersion(packageName string) entities.SemVer
	GetVerisons() map[string]entities.SemVer
	GetPackages() []string
}

type downloadState struct {
	mutex    sync.Mutex
	states   map[string]int
	versions map[string]entities.SemVer
	logger   logger.Logger

	downloadedCount int
	analysedCount   int
}

func NewDownloadState(fromVersions map[string]entities.SemVer, logger logger.Logger) DownloadState {
	return &downloadState{
		mutex:    sync.Mutex{},
		states:   make(map[string]int),
		versions: fromVersions,
		logger:   logger,

		downloadedCount: 0,
		analysedCount:   0,
	}
}

func (d *downloadState) GetState(packageName string) int {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	state, ok := d.states[packageName]
	if !ok {
		d.states[packageName] = NotStartedState
		return NotStartedState
	}

	return state
}

func (d *downloadState) SetState(packageName string, state int, version *entities.SemVer) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.states[packageName] = state

	if version != nil && version.Compare(ZERO_VERSION) > 0 {
		d.versions[packageName] = *version
	}
}

func (d *downloadState) GetDownloadedCount() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.downloadedCount
}

func (d *downloadState) IncrementDownloadedCount() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.downloadedCount++
}

func (d *downloadState) GetAnalysedCount() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.analysedCount
}

func (d *downloadState) IncrementAnalysedCount() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.analysedCount++
}

func (d *downloadState) GetLastVersion(packageName string) entities.SemVer {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.versions[packageName]
}

func (d *downloadState) GetVerisons() map[string]entities.SemVer {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.versions
}

func (d *downloadState) GetPackages() []string {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	packages := make([]string, 0, len(d.versions))
	for pkg := range d.versions {
		packages = append(packages, pkg)
	}

	return packages

}
