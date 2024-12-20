package repositories

import (
	context "context"
	"fmt"
	io "io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/npmoffline/internal/pkg/httpclient"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

func TestFetchMetadata(t *testing.T) {
	mockClient := httpclient.NewMockClient(t)
	repo := &npmRepository{client: mockClient, baseURL: "https://registry.npmjs.org"}

	// t.Run("Success", func(t *testing.T) {
	// 	packageName := "@mui/icons-material"
	// 	encodedPackageName := url.QueryEscape(packageName)
	// 	expectedURL := "https://registry.npmjs.org/" + encodedPackageName
	// 	body := `{
	// 	"versions": {
	// 		"1.0.0": {
	// 			"name": "@mui/icons-material",
	// 			"version": "1.0.0"
	// 		}
	// 	}
	// }`

	// 	mockClient.On("Do", mock.Anything, "GET", expectedURL, nil, mock.Anything).Return(mockResponse(http.StatusOK, body), nil).Once()

	// 	result, err := repo.FetchMetadata(context.Background(), packageName)

	// 	assert.NoError(t, err)
	// 	assert.Equal(t, 1, len(result))
	// 	assert.Equal(t, "@mui/icons-material", result[0].Name)
	// 	assert.Equal(t, "1.0.0", result[0].Version.String())
	// 	mockClient.AssertExpectations(t)
	// })

	t.Run("EncodesPackageName", func(t *testing.T) {
		packageName := "@mui/icons-material"
		encodedPackageName := url.QueryEscape(packageName)
		expectedURL := "https://registry.npmjs.org/" + encodedPackageName

		mockClient.On("Do", mock.Anything, "GET", expectedURL, nil, mock.Anything).Return(mockResponse(http.StatusOK, `{"versions": {}}`), nil).Once()

		_, err := repo.FetchMetadata(context.Background(), packageName)

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("UnexpectedStatusCode", func(t *testing.T) {
		mockClient.On("Do", mock.Anything, "GET", mock.Anything, nil, mock.Anything).Return(mockResponse(http.StatusNotFound, ``), nil).Once()

		_, err := repo.FetchMetadata(context.Background(), "test-package")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")
		mockClient.AssertExpectations(t)
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		mockClient.On("Do", mock.Anything, "GET", mock.Anything, nil, mock.Anything).Return(mockResponse(http.StatusOK, "invalid-json"), nil).Once()

		_, err := repo.FetchMetadata(context.Background(), "test-package")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode response")
		mockClient.AssertExpectations(t)
	})

	t.Run("EmptyResponse", func(t *testing.T) {
		mockClient.On("Do", mock.Anything, "GET", mock.Anything, nil, mock.Anything).Return(mockResponse(http.StatusOK, `{"versions": {}}`), nil).Once()

		result, err := repo.FetchMetadata(context.Background(), "test-package")

		assert.NoError(t, err)
		assert.Empty(t, result)
		mockClient.AssertExpectations(t)
	})

}

func TestDownloadTarballStream(t *testing.T) {
	mockClient := httpclient.NewMockClient(t)
	repo := &npmRepository{
		client: mockClient,
	}

	t.Run("Success", func(t *testing.T) {
		bodyBody := "tarball content"
		mockClient.On("Do", mock.Anything, "GET", "https://example.com/tarball.tgz", nil, mock.Anything).Return(mockResponse(http.StatusOK, bodyBody), nil).Once()

		result, err := repo.DownloadTarballStream(context.Background(), "https://example.com/tarball.tgz")
		assert.NoError(t, err)
		assert.NotNil(t, result)

		content, readErr := io.ReadAll(result)
		assert.NoError(t, readErr)
		assert.Equal(t, bodyBody, string(content))
		mockClient.AssertExpectations(t)
	})

	t.Run("Network error", func(t *testing.T) {
		mockClient.On("Do", mock.Anything, "GET", "https://example.com/tarball.tgz", nil, mock.Anything).Return(nil, fmt.Errorf("network error")).Once()

		result, err := repo.DownloadTarballStream(context.Background(), "https://example.com/tarball.tgz")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download tarball")
		assert.Nil(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("Unexpected status code", func(t *testing.T) {
		mockClient.On("Do", mock.Anything, "GET", "https://example.com/tarball.tgz", nil, mock.Anything).Return(mockResponse(http.StatusNotFound, ""), nil).Once()

		result, err := repo.DownloadTarballStream(context.Background(), "https://example.com/tarball.tgz")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")
		assert.Nil(t, result)
		mockClient.AssertExpectations(t)
	})

}

func mockResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
