package repositories

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	entities "github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/filesystem"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

// func TestWritePackageJSON(t *testing.T) {
// 	mockFS := filesystem.NewMockFileSystem(t)

// 	t.Run("success", func(t *testing.T) {
// 		repo := NewFileRepository("/tmp/test", mockFS)

// 		// Inputs
// 		packageName := "test-package"
// 		jsonData := map[string]interface{}{
// 			"name":    "test-package",
// 			"version": "1.0.0",
// 		}
// 		destDir := "/tmp/test/test-package"
// 		filePath := "/tmp/test/test-package/package.json"

// 		// Mock file
// 		mockFile := os.NewFile(0, "test-file")

// 		// Mocks
// 		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
// 		mockFS.On("Create", filePath).Return(mockFile, nil).Once()

// 		// Execution
// 		err := repo.WritePackageJSON(packageName, jsonData)

// 		// Assertions
// 		assert.NoError(t, err)
// 		mockFS.AssertCalled(t, "MkdirAll", destDir, os.ModePerm)
// 		mockFS.AssertCalled(t, "Create", filePath)

// 		// Verify the written JSON content
// 		var writtenData map[string]interface{}
// 		// json.Unmarshal(buffer.Bytes(), &writtenData)
// 		assert.Equal(t, jsonData, writtenData)

// 		mockFS.AssertExpectations(t)
// 	})

// 	t.Run("MkdirAllError", func(t *testing.T) {
// 		repo := NewFileRepository("/tmp/test2", mockFS)

// 		// Inputs
// 		packageName := "test-package"
// 		jsonData := map[string]interface{}{
// 			"name":    "test-package",
// 			"version": "1.0.0",
// 		}
// 		destDir := "/tmp/test2/test-package"

// 		// Mocks
// 		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(errors.New("mkdir error")).Once()

// 		// Execution
// 		err := repo.WritePackageJSON(packageName, jsonData)

// 		// Assertions
// 		assert.Error(t, err)
// 		assert.Contains(t, err.Error(), "mkdir error")
// 		mockFS.AssertCalled(t, "MkdirAll", destDir, os.ModePerm)
// 		mockFS.AssertNotCalled(t, "Create", mock.Anything)
// 		mockFS.AssertExpectations(t)
// 	})

// 	t.Run("CreateError", func(t *testing.T) {
// 		repo := NewFileRepository("/tmp/test3", mockFS)

// 		// Inputs
// 		packageName := "test-package"
// 		jsonData := map[string]interface{}{
// 			"name":    "test-package",
// 			"version": "1.0.0",
// 		}
// 		destDir := "/tmp/test3/test-package"
// 		filePath := "/tmp/test3/test-package/package.json"

// 		// Mocks
// 		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
// 		mockFS.On("Create", filePath).Return(nil, errors.New("create file error")).Once()

// 		// Execution
// 		err := repo.WritePackageJSON(packageName, jsonData)

// 		// Assertions
// 		assert.Error(t, err)
// 		assert.Contains(t, err.Error(), "create file error")
// 		mockFS.AssertCalled(t, "MkdirAll", destDir, os.ModePerm)
// 		mockFS.AssertCalled(t, "Create", filePath)
// 		mockFS.AssertExpectations(t)
// 	})

// 	t.Run("EncodeError", func(t *testing.T) {
// 		repo := NewFileRepository("/tmp/test4", mockFS)

// 		// Inputs
// 		packageName := "test-package"
// 		jsonData := make(chan int) // invalid JSON data type
// 		destDir := "/tmp/test4/test-package"
// 		filePath := "/tmp/test4/test-package/package.json"

// 		// Mock file
// 		mockFile := os.NewFile(0, "test-file")

// 		// Mocks
// 		mockFS.On("MkdirAll", destDir, os.ModePerm).Return(nil).Once()
// 		mockFS.On("Create", filePath).Return(mockFile, nil).Once()

// 		// Execution
// 		err := repo.WritePackageJSON(packageName, jsonData)

// 		// Assertions
// 		assert.Error(t, err)
// 		assert.Contains(t, err.Error(), "unable to write JSON data")
// 		mockFS.AssertCalled(t, "MkdirAll", destDir, os.ModePerm)
// 		mockFS.AssertCalled(t, "Create", filePath)
// 		mockFS.AssertExpectations(t)
// 	})
// }

func TestGetPackageDir(t *testing.T) {
	mockFS := filesystem.NewMockFileSystem(t)

	repo := fileRepository{
		baseDir: "/tmp/test",
		fs:      mockFS,
	}

	t.Run("valid package name", func(t *testing.T) {
		packageName := "test/package"
		expectedDir := filepath.Join("/tmp/test", "test", "package")

		result := repo.getPackageDir(packageName)

		assert.Equal(t, expectedDir, result)
	})

	t.Run("root package name", func(t *testing.T) {
		packageName := "root"
		expectedDir := filepath.Join("/tmp/test", "root")

		result := repo.getPackageDir(packageName)

		assert.Equal(t, expectedDir, result)
	})
}

func TestLoadOrCreateDownloadState(t *testing.T) {
	// Ack
	baseDir := "/tmp/test"

	// Mocks
	mockFS := filesystem.NewMockFileSystem(t)
	mockReader := filesystem.NewMockReader(t)

	repo := NewFileRepository(baseDir, mockFS, "")

	t.Run("success", func(t *testing.T) {

		// Ack
		status := map[string]entities.SemVer{
			"lodash":     {Major: 4, Minor: 17, Patch: 21, PreRelease: ""},
			"express":    {Major: 5, Minor: 0, Patch: 0, PreRelease: ""},
			"@mui/react": {Major: 17, Minor: 0, Patch: 2, PreRelease: ""},
		}

		// Mocks
		mockFS.On("Open", mock.Anything).Return(os.NewFile(0, "test-file"), nil).Once()
		mockFS.On("NewReader", mock.Anything).Return(mockReader).Once()
		mockReader.On("ReadString", mock.Anything).Return("lodash@4.17.21\n", nil).Once()
		mockReader.On("ReadString", mock.Anything).Return("express@5.0.0\n", nil).Once()
		mockReader.On("ReadString", mock.Anything).Return("@mui/react@17.0.2\n", nil).Once()
		mockReader.On("ReadString", mock.Anything).Return("", io.EOF).Once()

		// Appelle la méthode LoadOrCreateDownloadState
		result, err := repo.LoadOrCreateDownloadState()
		assert.NoError(t, err)
		assert.Equal(t, status, result)

	})

	t.Run("error", func(t *testing.T) {

		// Mocks
		mockFS.On("Open", mock.Anything).Return(nil, errors.New("file error")).Once()

		// Appelle la méthode LoadOrCreateDownloadState
		result, err := repo.LoadOrCreateDownloadState()
		assert.Error(t, err)
		assert.Equal(t, make(map[string]entities.SemVer), result)
	})

}

func TestSaveDownloadedVersions(t *testing.T) {
	// Ack
	baseDir := "/tmp/test"

	// Mocks
	mockFS := filesystem.NewMockFileSystem(t)
	mockWriter := filesystem.NewMockWriter(t)

	repo := NewFileRepository(baseDir, mockFS, "")

	t.Run("success", func(t *testing.T) {

		// Ack
		status := map[string]entities.SemVer{
			"lodash":  {Major: 4, Minor: 17, Patch: 21, PreRelease: ""},
			"express": {Major: 5, Minor: 0, Patch: 0, PreRelease: ""},
			"react":   {Major: 17, Minor: 0, Patch: 2, PreRelease: ""},
		}

		// Mocks
		mockFS.On("Create", mock.Anything).Return(os.NewFile(0, "test-file"), nil).Once()
		mockFS.On("NewWriter", mock.Anything).Return(mockWriter, nil).Once()
		mockWriter.On("WriteString", "lodash@4.17.21\n").Return(15, nil).Once()
		mockWriter.On("WriteString", "express@5.0.0\n").Return(15, nil).Once()
		mockWriter.On("WriteString", "react@17.0.2\n").Return(15, nil).Once()
		mockWriter.On("Flush").Return(nil).Once()

		// Appelle la méthode SaveDownloadedVersions
		err := repo.SaveDownloadedVersions(status)
		assert.NoError(t, err)

	})
}
