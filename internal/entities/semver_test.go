package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSemVer(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    SemVer
		expectError bool
	}{
		{
			name:     "Valid version",
			input:    "1.2.3",
			expected: SemVer{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:     "Valid version with leading zero",
			input:    "01.02.03",
			expected: SemVer{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:     "Valid version with release candidate",
			input:    "1.2.3-rc1",
			expected: SemVer{Major: 1, Minor: 2, Patch: 3, PreRelease: "rc1"},
		},
		{
			name:     "Valid version in development",
			input:    "1.9.0-dev.20160523-1.0",
			expected: SemVer{Major: 1, Minor: 9, Patch: 0, PreRelease: "dev.20160523-1.0"},
		},
		{
			name:        "Invalid format",
			input:       "1.2",
			expectError: true,
		},
		{
			name:        "Non-numeric major",
			input:       "a.2.3",
			expectError: true,
		},
		{
			name:        "Non-numeric minor",
			input:       "1.b.3",
			expectError: true,
		},
		{
			name:        "Non-numeric patch",
			input:       "1.2.c",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewSemVer(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name     string
		v1       SemVer
		v2       SemVer
		expected int
	}{
		{
			name:     "Equal versions",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0},
			expected: 0,
		},
		{
			name:     "v1 > v2 (major difference)",
			v1:       SemVer{Major: 2, Minor: 0, Patch: 0},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0},
			expected: 1,
		},
		{
			name:     "v1 < v2 (major difference)",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0},
			v2:       SemVer{Major: 2, Minor: 0, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 > v2 (minor difference)",
			v1:       SemVer{Major: 1, Minor: 1, Patch: 0},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0},
			expected: 1,
		},
		{
			name:     "v1 < v2 (minor difference)",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0},
			v2:       SemVer{Major: 1, Minor: 1, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 > v2 (patch difference)",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 1},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0},
			expected: 1,
		},
		{
			name:     "v1 < v2 (patch difference)",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 1},
			expected: -1,
		},
		{
			name:     "v1 < v2 (pre-release)",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: "rc1"},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 > v2 pre-release alphanumeric order",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: "rc1"},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: "alpha1"},
			expected: 1,
		},
		{
			name:     "v1 < v2 pre-release numeric order",
			v1:       SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: "2024"},
			v2:       SemVer{Major: 1, Minor: 0, Patch: 0, PreRelease: "rc2"},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.v1.Compare(tt.v2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPreRelease(t *testing.T) {
	tests := []struct {
		name     string
		version  SemVer
		expected bool
	}{
		{
			name:     "No pre-release",
			version:  SemVer{Major: 1, Minor: 2, Patch: 3},
			expected: false,
		},
		{
			name:     "With pre-release",
			version:  SemVer{Major: 1, Minor: 2, Patch: 3, PreRelease: "rc1"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.IsPreRelease()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		version  SemVer
		expected string
	}{
		{
			name:     "No pre-release",
			version:  SemVer{Major: 1, Minor: 2, Patch: 3},
			expected: "1.2.3",
		},
		{
			name:     "With pre-release",
			version:  SemVer{Major: 1, Minor: 2, Patch: 3, PreRelease: "rc1"},
			expected: "1.2.3-rc1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
