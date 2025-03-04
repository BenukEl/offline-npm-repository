package repositories

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/npmoffline/internal/pkg/filesystem"
)

// LocalNpmRepository defines operations for writing tarballs and metadata,
// and for managing the downloaded packages state.
type LocalNpmRepository interface {
	WriteTarball(packageName, version string, reader io.ReadCloser) error
	WritePackageJSON(packageName string, reader io.ReadCloser) (io.ReadCloser, error)
	LoadDownloadedPackagesState() ([]string, time.Time, error)
	SaveDownloadedPackagesState(packages []string, lastSync time.Time) error
}

// localNpmRepo implements LocalNpmRepository.
type localNpmRepo struct {
	npmDirPath    string
	stateFilePath string
	fs            filesystem.FileSystem
}

// NewLocalNpmRepository creates a new instance of LocalNpmRepository.
// - baseDir is the root directory for storing package files.
// - stateFilePath is the file path where the downloaded packages state is stored.
func NewLocalNpmRepository(baseDir string, fs filesystem.FileSystem, stateFilePath string) LocalNpmRepository {
	return &localNpmRepo{
		npmDirPath:    baseDir,
		stateFilePath: stateFilePath,
		fs:            fs,
	}
}

// WriteTarball writes the tarball data to the appropriate directory.
func (r *localNpmRepo) WriteTarball(packageName, version string, reader io.ReadCloser) error {
	destDir := r.getPackageDirectory(packageName)
	if err := r.fs.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Use path.Base to handle scoped packages correctly.
	filePath := filepath.Join(destDir, fmt.Sprintf("%s-%s.tgz", path.Base(packageName), version))
	file, err := r.fs.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	if _, err := r.fs.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write tarball data: %w", err)
	}

	return nil
}

// teeReadCloser wraps a TeeReader and a WriteCloser to implement io.ReadCloser.
type teeReadCloser struct {
	tee io.Reader
	w   io.WriteCloser
}

func (t *teeReadCloser) Read(p []byte) (n int, err error) {
	return t.tee.Read(p)
}

func (t *teeReadCloser) Close() error {
	return t.w.Close()
}

// WritePackageJSON writes the package.json file to the package directory.
// It returns an io.ReadCloser that provides the read data while writing it to disk.
func (r *localNpmRepo) WritePackageJSON(packageName string, reader io.ReadCloser) (io.ReadCloser, error) {
	destDir := r.getPackageDirectory(packageName)
	if err := r.fs.MkdirAll(destDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	filePath := filepath.Join(destDir, "package.json")
	file, err := r.fs.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	tee := r.fs.TeeReader(reader, file)
	return &teeReadCloser{tee: tee, w: file}, nil
}

// LoadDownloadedPackagesState loads the downloaded packages state from disk.
// The state file format is expected to have the sync date in the first line,
// prefixed with "Last sync: ", followed by one package name per line.
func (r *localNpmRepo) LoadDownloadedPackagesState() ([]string, time.Time, error) {
	file, err := r.fs.Open(r.stateFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// If the file does not exist, return an empty state.
			return []string{}, time.Time{}, nil
		}
		return nil, time.Time{}, fmt.Errorf("failed to open file %s: %w", r.stateFilePath, err)
	}
	defer file.Close()

	scanner := r.fs.NewScanner(file)
	var packages []string
	var lastSync time.Time

	if scanner.Scan() {
		firstLine := scanner.Text()
		const prefix = "Last sync: "
		if !strings.HasPrefix(firstLine, prefix) {
			return nil, time.Time{}, fmt.Errorf("invalid state file format: missing date prefix")
		}
		dateStr := strings.TrimSpace(firstLine[len(prefix):])
		lastSync, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("invalid date format in state file: %v", err)
		}
	} else {
		// In the rare case the file is empty, initialize lastSync to zero value.
		lastSync = time.Time{}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			packages = append(packages, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, time.Time{}, err
	}

	return packages, lastSync, nil
}

// SaveDownloadedPackagesState saves the downloaded packages state to disk.
// It writes the sync date on the first line and one package name per subsequent line.
func (r *localNpmRepo) SaveDownloadedPackagesState(packages []string, lastSync time.Time) error {
	file, err := r.fs.Create(r.stateFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", r.stateFilePath, err)
	}
	defer file.Close()

	writer := r.fs.NewWriter(file)

	// Write the sync date
	if _, err := writer.WriteString(fmt.Sprintf("Last sync: %s\n", lastSync.Format(time.RFC3339))); err != nil {
		return fmt.Errorf("failed to write sync date to file %s: %w", r.stateFilePath, err)
	}
	// Write each package name on a new line.
	for _, pkg := range packages {
		if _, err := writer.WriteString(pkg + "\n"); err != nil {
			return fmt.Errorf("failed to write package to file %s: %w", r.stateFilePath, err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush data to file %s: %w", r.stateFilePath, err)
	}

	return nil
}

// getPackageDirectory returns the directory path for the given package.
// It replaces slashes in package names to create nested directories.
func (r *localNpmRepo) getPackageDirectory(packageName string) string {
	return filepath.Join(r.npmDirPath, filepath.FromSlash(packageName))
}
