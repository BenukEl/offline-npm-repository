package entities

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SemVer represents a Semantic Versioning structure.
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
}

// NewSemVer creates a Semantic Version object from a string.
func NewSemVer(version string) (SemVer, error) {
	regex := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-(.*))?$`)
	matches := regex.FindStringSubmatch(version)

	if matches == nil {
		return SemVer{}, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return SemVer{}, fmt.Errorf("invalid patch version: %w", err)
	}

	preRelease := matches[4] // Optional pre-release tag

	return SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
	}, nil
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, and 1 if v > other.
func (v SemVer) Compare(other SemVer) int {
	if v.Major != other.Major {
		if v.Major > other.Major {
			return 1
		}
		return -1
	}
	if v.Minor != other.Minor {
		if v.Minor > other.Minor {
			return 1
		}
		return -1
	}
	if v.Patch != other.Patch {
		if v.Patch > other.Patch {
			return 1
		}
		return -1
	}

	// Compare pre-release versions (if they exist)
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1 // No pre-release > pre-release
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1 // Pre-release < no pre-release
	}
	if v.PreRelease != "" && other.PreRelease != "" {
		return strings.Compare(v.PreRelease, other.PreRelease)
	}

	return 0 // Versions are equal
}

// IsPreRelease returns true if the version is a pre-release.
func (v SemVer) IsPreRelease() bool {
	return v.PreRelease != ""
}

// String returns the string representation of the version.
func (v SemVer) String() string {
	version := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		version += "-" + v.PreRelease
	}
	return version
}
