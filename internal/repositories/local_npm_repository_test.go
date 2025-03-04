package repositories

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Dummy types for testing ---

// fakeWriter simulates a filesystem.Writer.
type fakeWriter struct {
	content        string
	failOnWrite    bool
	failOnWritePkg bool
	failOnFlush    bool
}

func (fw *fakeWriter) WriteString(s string) (int, error) {
	if fw.failOnWrite {
		return 0, fmt.Errorf("write sync error")
	}
	if fw.failOnWritePkg {
		fw.failOnWrite = true
	}
	fw.content += s
	return len(s), nil
}

func (fw *fakeWriter) Flush() error {
	if fw.failOnFlush {
		return fmt.Errorf("flush error")
	}
	return nil
}

// fakeScanner simulates a filesystem.Scanner.
type fakeScanner struct {
	lines []string
	index int
	err   error
}

func (s *fakeScanner) Scan() bool {
	if s.index < len(s.lines) {
		s.index++
		return true
	}
	return false
}

func (s *fakeScanner) Text() string {
	// returns the current line (note that Scan() has already incremented index)
	return s.lines[s.index-1]
}

func (s *fakeScanner) Err() error {
	return s.err
}

func TestWriteTarball(t *testing.T) {
	packageName := "lodash"
	version := "1.0.0"
	readerContent := "tarball data"

	destDir := filepath.Join("base", filepath.FromSlash(packageName))
	filePath := filepath.Join(destDir, fmt.Sprintf("%s-%s.tgz", path.Base(packageName), version))

	mockFile := os.NewFile(0, "test-file")
	mockFS := filesystem.NewMockFileSystem(t)

	t.Run("MkdirAll fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")
		reader := io.NopCloser(strings.NewReader("tarball data"))

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(fmt.Errorf("mkdir error")).Once()

		err := repo.WriteTarball(packageName, version, reader)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
		mockFS.AssertExpectations(t)
	})

	t.Run("Create fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")
		reader := io.NopCloser(strings.NewReader("tarball data"))

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
		mockFS.On("Create", filePath).Return(nil, fmt.Errorf("create error")).Once()

		err := repo.WriteTarball(packageName, version, reader)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create file")
		mockFS.AssertExpectations(t)
	})

	t.Run("Copy fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		reader := io.NopCloser(strings.NewReader(readerContent))
		// Mock file

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
		mockFS.On("Create", filePath).Return(mockFile, nil).Once()
		mockFS.On("Copy", mockFile, reader).Return(int64(0), fmt.Errorf("copy error")).Once()

		err := repo.WriteTarball(packageName, version, reader)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write tarball data")
		mockFS.AssertExpectations(t)
	})

	t.Run("Success", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		reader := io.NopCloser(strings.NewReader(readerContent))

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
		mockFS.On("Create", filePath).Return(mockFile, nil).Once()
		mockFS.On("Copy", mockFile, reader).Return(int64(len(readerContent)), nil).Once()

		err := repo.WriteTarball(packageName, version, reader)
		require.NoError(t, err)
		mockFS.AssertExpectations(t)
	})
}
func TestWritePackageJSON(t *testing.T) {
	packageName := "react"
	jsonContent := `{"name": "react"}`

	reader := io.NopCloser(strings.NewReader(jsonContent))
	destDir := filepath.Join("base", filepath.FromSlash(packageName))
	filePath := filepath.Join(destDir, "package.json")

	mockFile := os.NewFile(0, "test-file")
	mockFS := filesystem.NewMockFileSystem(t)

	t.Run("MkdirAll fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(fmt.Errorf("mkdir error")).Once()

		result, err := repo.WritePackageJSON(packageName, reader)
		require.Error(t, err)
		assert.Nil(t, result)
		mockFS.AssertExpectations(t)
	})

	t.Run("Create fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
		mockFS.On("Create", filePath).Return(nil, fmt.Errorf("create error")).Once()

		result, err := repo.WritePackageJSON(packageName, reader)
		require.Error(t, err)
		assert.Nil(t, result)
		mockFS.AssertExpectations(t)
	})

	t.Run("Success", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
		mockFS.On("Create", filePath).Return(mockFile, nil).Once()
		// Simulate TeeReader by returning the same reader.
		mockFS.On("TeeReader", reader, mockFile).Return(reader).Once()

		result, err := repo.WritePackageJSON(packageName, reader)
		require.NoError(t, err)
		require.NotNil(t, result)
		data, _ := io.ReadAll(result)
		assert.Equal(t, jsonContent, string(data))
		mockFS.AssertExpectations(t)
	})
}

func TestLoadDownloadedPackagesState(t *testing.T) {
	mockScanner := func(content string, err bool) (*fakeScanner, io.ReadCloser) {
		lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		mockFile := os.NewFile(0, "test-file")
		scan := &fakeScanner{lines: lines}
		if err {
			scan.err = fmt.Errorf("scanner error")
		}
		return scan, mockFile
	}

	mockFS := filesystem.NewMockFileSystem(t)

	t.Run("Open fails with error", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")
		openErr := fmt.Errorf("open error")

		mockFS.On("Open", "state.txt").Return(nil, openErr).Once()

		_, _, err := repo.LoadDownloadedPackagesState()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open file")
		mockFS.AssertExpectations(t)
	})

	t.Run("File does not exist", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Open", "state.txt").Return(nil, os.ErrNotExist).Once()

		pkgs, syncTime, err := repo.LoadDownloadedPackagesState()
		require.NoError(t, err)
		assert.Empty(t, pkgs)
		assert.True(t, syncTime.IsZero())
		mockFS.AssertExpectations(t)
	})

	t.Run("Invalid prefix in first line", func(t *testing.T) {
		content := "Invalid sync line\npackage1\n"
		fakeScan, dummyFile := mockScanner(content, true)

		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Open", "state.txt").Return(dummyFile, nil).Once()
		mockFS.On("NewScanner", dummyFile).Return(fakeScan).Once()

		_, _, err := repo.LoadDownloadedPackagesState()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid state file format")
		mockFS.AssertExpectations(t)
	})

	t.Run("Invalid date format in first line", func(t *testing.T) {
		content := "Last sync: invalid-date\npackage1\n"
		fakeScan, dummyFile := mockScanner(content, true)

		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Open", "state.txt").Return(dummyFile, nil).Once()
		mockFS.On("NewScanner", dummyFile).Return(fakeScan).Once()

		_, _, err := repo.LoadDownloadedPackagesState()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid date format")
		mockFS.AssertExpectations(t)
	})

	t.Run("Scanner error", func(t *testing.T) {
		content := "Last sync: 2025-03-03T23:20:12Z\npackage1\n"
		fakeScan, dummyFile := mockScanner(content, true)

		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Open", "state.txt").Return(dummyFile, nil).Once()
		mockFS.On("NewScanner", dummyFile).Return(fakeScan).Once()

		_, _, err := repo.LoadDownloadedPackagesState()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scanner error")
		mockFS.AssertExpectations(t)
	})

	t.Run("Success with packages", func(t *testing.T) {
		syncTime := time.Now().UTC().Round(time.Second)
		content := fmt.Sprintf("Last sync: %s\npackage1\npackage2\n", syncTime.Format(time.RFC3339))
		fakeScan, dummyFile := mockScanner(content, false)

		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Open", "state.txt").Return(dummyFile, nil).Once()
		mockFS.On("NewScanner", dummyFile).Return(fakeScan).Once()

		pkgs, readSyncTime, err := repo.LoadDownloadedPackagesState()
		require.NoError(t, err)
		assert.Equal(t, []entities.RetrievePackage{
			entities.NewRetrievePackage("package1"),
			entities.NewRetrievePackage("package2"),
		}, pkgs)
		assert.True(t, readSyncTime.Equal(syncTime))
		mockFS.AssertExpectations(t)
	})

	t.Run("Success with no packages", func(t *testing.T) {
		syncTime := time.Now().UTC().Round(time.Second)
		content := fmt.Sprintf("Last sync: %s\n", syncTime.Format(time.RFC3339))
		fakeScan, dummyFile := mockScanner(content, false)

		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Open", "state.txt").Return(dummyFile, nil).Once()
		mockFS.On("NewScanner", dummyFile).Return(fakeScan).Once()

		pkgs, readSyncTime, err := repo.LoadDownloadedPackagesState()
		require.NoError(t, err)
		assert.Empty(t, pkgs)
		assert.True(t, readSyncTime.Equal(syncTime))
		mockFS.AssertExpectations(t)
	})
}

func TestSaveDownloadedPackagesState(t *testing.T) {
	onePkg := []entities.RetrievePackage{
		entities.NewRetrievePackage("pkg1|alpha"),
	}
	twoPkgs := []entities.RetrievePackage{
		entities.NewRetrievePackage("pkg1"),
		entities.NewRetrievePackage("pkg2|toto"),
	}

	mockFile := os.NewFile(0, "test-file")
	mockFS := filesystem.NewMockFileSystem(t)

	t.Run("Create fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		createErr := fmt.Errorf("create error")
		mockFS.On("Create", "state.txt").Return(nil, createErr).Once()

		err := repo.SaveDownloadedPackagesState(onePkg, time.Now().UTC())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create file")
		mockFS.AssertExpectations(t)
	})

	t.Run("Write sync date fails", func(t *testing.T) {
		mockFS := filesystem.NewMockFileSystem(t)
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")
		mockFS.On("Create", "state.txt").Return(mockFile, nil).Once()
		// Return a writer that fails on sync date write.
		fw := &fakeWriter{failOnWrite: true}
		mockFS.On("NewWriter", mockFile).Return(fw).Once()

		err := repo.SaveDownloadedPackagesState(onePkg, time.Now().UTC())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write sync date")
		mockFS.AssertExpectations(t)
	})

	t.Run("Write package fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Create", "state.txt").Return(mockFile, nil).Once()
		// Use a fake writer that will fail when writing a package line.
		fw := &fakeWriter{failOnWritePkg: true}
		// For this test, we simulate success for sync date then error on package.
		mockFS.On("NewWriter", mockFile).Return(fw).Once()
		err := repo.SaveDownloadedPackagesState(onePkg, time.Now().UTC())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write package")
		mockFS.AssertExpectations(t)
	})

	t.Run("Flush fails", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Create", "state.txt").Return(mockFile, nil).Once()
		fw := &fakeWriter{failOnFlush: true}
		mockFS.On("NewWriter", mockFile).Return(fw).Once()

		err := repo.SaveDownloadedPackagesState(onePkg, time.Now().UTC())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to flush data")
		mockFS.AssertExpectations(t)
	})

	t.Run("Success", func(t *testing.T) {
		repo := NewLocalNpmRepository("base", mockFS, "state.txt")

		mockFS.On("Create", "state.txt").Return(mockFile, nil).Once()
		fw := &fakeWriter{}
		mockFS.On("NewWriter", mockFile).Return(fw).Once()
		syncTime := time.Now().UTC().Round(time.Second)

		err := repo.SaveDownloadedPackagesState(twoPkgs, syncTime)
		require.NoError(t, err)
		expected := fmt.Sprintf("Last sync: %s\npkg1\npkg2|toto\n", syncTime.Format(time.RFC3339))
		assert.Equal(t, expected, fw.content)
		mockFS.AssertExpectations(t)
	})
}
