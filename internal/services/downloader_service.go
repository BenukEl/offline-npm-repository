package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
)

const (
	channelBufferSize    = 100_000
	workerInactivityTime = 10 * time.Second
)

// StartOptions contains the options to start the download service
type StartOptions struct {
	MetadataWorkers       int
	DownloadWorkers       int
	UpdateLocalRepository bool
}

type DownloadService interface {
	Start(ctx context.Context, packageList []string, options StartOptions)
}

type downloadService struct {
	npmRepo       repositories.NpmRepository
	fileRepo      repositories.FileRepository
	logger        logger.Logger
	metadataChan  chan string
	downloadChan  chan entities.NpmPackage
	wgMetadata    *sync.WaitGroup
	wgDownload    *sync.WaitGroup
	downloadState DownloadState
}

func NewDownloadService(npmRepo repositories.NpmRepository, fileRepo repositories.FileRepository, log logger.Logger) *downloadService {
	fromVersions, err := fileRepo.LoadOrCreateDownloadState()
	if err != nil {
		log.Error("Failed to load downloaded versions: %w", err)
		fromVersions = make(map[string]entities.SemVer)
	}

	s := &downloadService{
		npmRepo:       npmRepo,
		fileRepo:      fileRepo,
		logger:        log,
		downloadState: NewDownloadState(fromVersions, log),
		wgMetadata:    &sync.WaitGroup{},
		wgDownload:    &sync.WaitGroup{},
		metadataChan:  make(chan string, channelBufferSize),
		downloadChan:  make(chan entities.NpmPackage, channelBufferSize),
	}

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			ana := s.downloadState.GetAnalysedCount()
			dl := s.downloadState.GetDownloadedCount()
			log.Info("Analysed: %d/%d, Downloaded: %d/%d", ana, ana+len(s.metadataChan), dl, dl+len(s.downloadChan))
		}
	}()

	return s
}

func (s *downloadService) Start(ctx context.Context, packageList []string, options StartOptions) {
	s.logger.Info("Starting download service")
	defer s.logger.Info("Download service stopped")

	// If UpdateLocalRepository is true, add packages from the downloadState to the packageList.
	if options.UpdateLocalRepository {
		s.logger.Info("Updating local repository: adding packages from downloadState to package list")
		packagesFromState := s.downloadState.GetPackages()
		packageList = append(packageList, packagesFromState...)
	}

	// Start metadata workers with configured count
	for i := 0; i < options.MetadataWorkers; i++ {
		s.wgMetadata.Add(1)
		go s.metadataWorker(ctx, i)
	}

	// Start download workers with configured count
	for i := 0; i < options.DownloadWorkers; i++ {
		s.wgDownload.Add(1)
		go s.downloadWorker(ctx, i)
	}

	// Enqueue packages
	for _, pkg := range packageList {
		s.metadataChan <- pkg
	}

	s.wgMetadata.Wait()
	s.logger.Info("Metadata workers finished")
	close(s.metadataChan)

	s.wgDownload.Wait()
	s.logger.Info("Download workers finished")
	close(s.downloadChan)

	// Saves the downloaded versions
	s.fileRepo.SaveDownloadedVersions(s.downloadState.GetVerisons())
}

// metadataWorker manages the metadata retrieval
func (s *downloadService) metadataWorker(ctx context.Context, workerIndex int) {
	defer s.wgMetadata.Done()

	s.logger.Debug("[meta_#%d] Worker started", workerIndex)

	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("[meta_#%d] Received context cancellation", workerIndex)
			return
		case pkgName := <-s.metadataChan:
			if pkgName == "" {
				continue
			}
			if err := s.retrieveMetadata(ctx, pkgName, workerIndex); err != nil {
				s.logger.Error("[meta_#%d] Failed to retrieve metadata for %s: %w", workerIndex, pkgName, err)
			}
		case <-time.After(workerInactivityTime):
			s.logger.Debug("[meta_#%d] Worker stopped due to inactivity", workerIndex)
			return
		}
	}
}

// downloadWorker manages the download of tarballs
func (s *downloadService) downloadWorker(ctx context.Context, workerIndex int) {
	defer s.wgDownload.Done()

	s.logger.Debug("[dl_#%d] Worker started", workerIndex)

	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("[dl_#%d] Worker received context cancellation", workerIndex)
			return
		case pkg := <-s.downloadChan:
			if pkg.Name == "" {
				continue
			}
			if err := s.downloadTarball(ctx, pkg, workerIndex); err != nil {
				s.logger.Error("Failed to download tarball for %s: %w", pkg.Name, err)
			}
		case <-time.After(workerInactivityTime):
			s.logger.Debug("[dl_#%d] Worker stopped due to inactivity", workerIndex)
			return
		}
	}
}

// retrieveMetadata retrieves the metadata for a package
func (s *downloadService) retrieveMetadata(ctx context.Context, packageName string, workerIndex int) error {
	if s.downloadState.GetState(packageName) != AnalysingState && s.downloadState.GetState(packageName) != NotStartedState {
		s.logger.Debug("Package %s already processed", packageName)
		return nil
	}

	s.logger.Debug("[meta_#%d] Fetching metadata for package: %s", workerIndex, packageName)
	reader, err := s.npmRepo.FetchMetadata(ctx, packageName)
	if err != nil {
		s.logger.Error("Failed to fetch metadata for %s: %w", packageName, err)
		return err
	}
	defer reader.Close()

	teeReader, err := s.fileRepo.WritePackageJSON(packageName, reader)
	if err != nil {
		return fmt.Errorf("failed to write package.json for package %s. Err: %w", packageName, err)
	}
	defer teeReader.Close()

	packages, err := s.npmRepo.DecodeNpmPackages(teeReader)
	if err != nil {
		return fmt.Errorf("failed to decode npm packages for packakge %s. Err: %w", packageName, err)
	}

	lastVersion := s.downloadState.GetLastVersion(packageName)

	// Filter packages that are greater than the last version
	filteredPackages := filterPackages(packages, lastVersion)

	var highestVersion entities.SemVer
	for _, pkg := range filteredPackages {
		if pkg.Version.Compare(highestVersion) > 0 {
			highestVersion = pkg.Version
		}

		if s.logger.IsDebug() {
			s.logger.Debug("[meta_#%d] Enqueueing package %s:%s for download", workerIndex, pkg.Name, pkg.Version.String())
		}

		s.downloadChan <- pkg

		for _, dep := range pkg.Dependencies {
			if s.downloadState.GetState(dep) == NotStartedState {
				s.downloadState.SetState(dep, AnalysingState, nil)
				s.metadataChan <- dep
			}
		}
		for _, dep := range pkg.PeerDeps {
			if s.downloadState.GetState(dep) == NotStartedState {
				s.downloadState.SetState(dep, AnalysingState, nil)
				s.metadataChan <- dep
			}
		}
	}

	s.logger.Debug("[meta_#%d] Processed package %s... %d versions to download (latest: %s)", workerIndex, packageName, len(filteredPackages), highestVersion)
	s.downloadState.SetState(packageName, AnalysedState, &highestVersion)
	s.downloadState.IncrementAnalysedCount()

	return nil
}

// downloadTarball downloads the tarball for a package
func (s *downloadService) downloadTarball(ctx context.Context, pkg entities.NpmPackage, workerIndex int) error {
	if s.logger.IsDebug() {
		s.logger.Debug("[dl_#%d] Downloading tarball for package %s:%s", workerIndex, pkg.Name, pkg.Version.String())
	}

	reader, err := s.npmRepo.DownloadTarballStream(ctx, pkg.Url)
	if err != nil {
		s.logger.Error("Failed to download tarball for %s:%s. Err:%w", pkg.Name, pkg.Version.String(), err)
		return err
	}
	defer reader.Close()

	// Write the tarball to the file system
	if err := s.fileRepo.WriteTarball(pkg.Name, pkg.Version.String(), reader); err != nil {
		s.logger.Error("Failed to write tarball for %s: %w", pkg.Name, err)
		return err
	}

	s.downloadState.IncrementDownloadedCount()
	if s.logger.IsDebug() {
		s.logger.Debug("[dl_#%d] Successfully downloaded tarball for package %s:%s", workerIndex, pkg.Name, pkg.Version.String())
	}

	return nil
}

// filterPackages filtre pre-release versions and versions that are less than the last version
func filterPackages(packages []entities.NpmPackage, lastVersion entities.SemVer) []entities.NpmPackage {
	var filteredPackages []entities.NpmPackage
	for _, pkg := range packages {
		if pkg.Version.PreRelease != "" {
			// Exclude pre-release versions
			continue
		}

		if pkg.Version.Compare(lastVersion) > 0 {
			filteredPackages = append(filteredPackages, pkg)
		}
	}
	return filteredPackages
}
