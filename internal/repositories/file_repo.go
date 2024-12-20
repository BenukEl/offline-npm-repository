package repositories

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	entities "github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/filesystem"
)

type FileRepository interface {
	WriteTarball(packageName, version string, reader io.ReadCloser) error
	WritePackageJSON(packageName string, reader io.ReadCloser) (io.ReadCloser, error)
	LoadOrCreateDownloadState() (map[string]entities.SemVer, error)
	SaveDownloadedVersions(dlVersions map[string]entities.SemVer) error
}

// FileRepository handles writing tarballs and metadata to disk.
type fileRepository struct {
	baseDir   string
	statePath string
	fs        filesystem.FileSystem
}

// NewFileRepository creates a new instance of FileRepository.
func NewFileRepository(baseDir string, fs filesystem.FileSystem, statePath string) FileRepository {
	return &fileRepository{
		baseDir:   baseDir,
		fs:        fs,
		statePath: statePath,
	}
}

// WriteTarball writes the tarball data to the appropriate directory.
func (r *fileRepository) WriteTarball(packageName, version string, reader io.ReadCloser) error {
	destDir := r.getPackageDir(packageName)
	if err := r.fs.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

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

type teeReadCloaser struct {
	tee io.Reader
	w   io.WriteCloser
}

func (t *teeReadCloaser) Read(p []byte) (n int, err error) {
	return t.tee.Read(p)
}

func (t *teeReadCloaser) Close() error {
	return t.w.Close()
}

// WritePackageJSON writes the package.json file to the appropriate directory.
func (r *fileRepository) WritePackageJSON(packageName string, reader io.ReadCloser) (io.ReadCloser, error) {
	destDir := r.getPackageDir(packageName)
	if err := r.fs.MkdirAll(destDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	filePath := filepath.Join(destDir, "package.json")
	file, err := r.fs.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	tee := r.fs.TeeReader(reader, file)

	return &teeReadCloaser{tee: tee, w: file}, nil
}

// LoadOrCreateDownloadState loads the last version download by package from the disk.
func (r *fileRepository) LoadOrCreateDownloadState() (map[string]entities.SemVer, error) {
	// Load the downloaded versions from a file
	file, err := r.fs.Open(r.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]entities.SemVer), nil
		}
		return make(map[string]entities.SemVer), fmt.Errorf("failed to open file %s: %w", r.statePath, err)
	}
	defer file.Close()

	// Read the file line by line and parse the custom format
	reader := r.fs.NewReader(file)
	status := make(map[string]entities.SemVer)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return make(map[string]entities.SemVer), fmt.Errorf("failed to read file %s: %w", r.statePath, err)
		}

		// Parse the custom format "
		lastAt := strings.LastIndex(line, "@")
		if lastAt == -1 {
			return make(map[string]entities.SemVer), fmt.Errorf("invalid format, '@' not found: %s", line)
		}

		pkg := line[:lastAt]
		version := line[lastAt+1 : len(line)-1]

		semVer, err := entities.NewSemVer(version)
		if err != nil {
			return make(map[string]entities.SemVer), fmt.Errorf("failed to parse version %s: %w", version, err)
		}
		status[pkg] = semVer
	}

	return status, nil
}

// SaveDownloadedVersions saves the last version downloaded by package to the disk in a custom format.
func (r *fileRepository) SaveDownloadedVersions(status map[string]entities.SemVer) error {
	// Détermine le chemin du fichier
	file, err := r.fs.Create(r.statePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", r.statePath, err)
	}
	defer file.Close()

	// Parcourt la map et écrit chaque entrée dans le fichier au format personnalisé
	writer := r.fs.NewWriter(file)
	for pkg, version := range status {
		// Construire le format personnalisé "package@major.minor.patch"
		line := fmt.Sprintf("%s@%s\n", pkg, version.String())
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write to file %s: %w", r.statePath, err)
		}
	}

	// S'assure que tout est bien écrit sur le disque
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush data to file %s: %w", r.statePath, err)
	}

	return nil
}

// getPackageDir determines the directory path for a package.
func (r *fileRepository) getPackageDir(packageName string) string {
	// Replace slashes in package names to create subdirectories
	return filepath.Join(r.baseDir, filepath.FromSlash(packageName))
}
