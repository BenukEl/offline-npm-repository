package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/npmoffline/internal/entities"
	"github.com/npmoffline/internal/pkg/httpclient"
	"github.com/npmoffline/internal/pkg/logger"
)

// Dist represents the distribution metadata for a package.
type Dist struct {
	Shasum    string `json:"shasum"`
	Tarball   string `json:"tarball"`
	Integrity string `json:"integrity"`
}

// NpmPackageMetadata represents the metadata for an NPM package version.
type NpmPackageMetadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	PeerDeps     map[string]string `json:"peerDependencies,omitempty"`
	Dist         Dist              `json:"dist"`
}

// ToNpmPackage converts the metadata to an NpmPackage entity.
// The date parameter is used to set the release date of the package.
func (m *NpmPackageMetadata) ToNpmPackage(date time.Time) (entities.NpmPackage, error) {
	version, err := entities.NewSemVer(m.Version)
	if err != nil {
		return entities.NpmPackage{}, fmt.Errorf("failed to convert version: %v", err)
	}

	// Convert dependencies and peer dependencies to a slice of strings
	var deps []string
	for dep := range m.Dependencies {
		deps = append(deps, dep)
	}

	var peerDeps []string
	for dep := range m.PeerDeps {
		peerDeps = append(peerDeps, dep)
	}

	return entities.NpmPackage{
		Name:         m.Name,
		Version:      version,
		ReleaseDate:  date,
		Dependencies: deps,
		PeerDeps:     peerDeps,
		Integrity:    m.Dist.Integrity,
		Url:          m.Dist.Tarball,
	}, nil
}

// NpmResponse represents the full response for an NPM package.
type NpmResponse struct {
	ID       string                        `json:"_id"`
	Rev      string                        `json:"_rev"`
	Name     string                        `json:"name"`
	Versions map[string]NpmPackageMetadata `json:"versions"`
	Time     map[string]time.Time          `json:"time"`
}

type NpmRepository interface {
	FetchMetadata(ctx context.Context, packageName string) (io.ReadCloser, error)
	DownloadTarballStream(ctx context.Context, tarballURL string) (io.ReadCloser, error)
	DecodeNpmPackages(r io.Reader) ([]entities.NpmPackage, error)
}

// npmRepository handles interactions with the NPM registry.
type npmRepository struct {
	baseURL string
	client  httpclient.Client
	logger  logger.Logger
}

// NewnpmRepository creates a new instance of npmRepository.
func NewNpmRepository(baseURL string, client httpclient.Client, log logger.Logger) NpmRepository {
	return &npmRepository{
		baseURL: baseURL,
		client:  client,
		logger:  log,
	}
}

// FetchMetadata retrieves the metadata of a package from the NPM registry.
func (r *npmRepository) FetchMetadata(ctx context.Context, packageName string) (io.ReadCloser, error) {
	// Encode the package name to make it safe for URL usage
	encodedPackageName := url.QueryEscape(packageName)
	url := fmt.Sprintf("%s/%s", r.baseURL, encodedPackageName)

	resp, err := r.client.Do(ctx, "GET", url, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %v", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close() // Important to close the body in case of errors
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// DownloadTarballStream downloads a tarball file and returns a stream (io.ReadCloser).
func (r *npmRepository) DownloadTarballStream(ctx context.Context, tarballURL string) (io.ReadCloser, error) {
	resp, err := r.client.Do(ctx, "GET", tarballURL, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download tarball: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() // Important to close the body in case of errors
		return nil, fmt.Errorf("unexpected status code %d while downloading tarball", resp.StatusCode)
	}

	// Return the response body as a ReadCloser
	return resp.Body, nil
}

// DecodeNpmPackages decodes the NPM packages from a reader.
func (r *npmRepository) DecodeNpmPackages(reader io.Reader) ([]entities.NpmPackage, error) {
	var metadata NpmResponse
	if err := json.NewDecoder(reader).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	var packages []entities.NpmPackage
	for _, releasedPackage := range metadata.Versions {
		pkg, err := releasedPackage.ToNpmPackage(metadata.Time[releasedPackage.Version])
		if err != nil {
			return nil, fmt.Errorf("failed to convert metadata: %v", err)
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}
