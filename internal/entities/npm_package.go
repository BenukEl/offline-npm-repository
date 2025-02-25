package entities

import (
	"regexp"
	"strings"
)

type NpmPackage struct {
	Name         string   `json:"name"`
	Version      SemVer   `json:"version"`
	Dependencies []string `json:"dependencies"`
	PeerDeps     []string `json:"peerDeps"`
	Integrity    string   `json:"integrity"`
	Url          string   `json:"url"`
}

type RetrievePackage struct {
	Name              string
	allowedPreVersion *regexp.Regexp
}

func NewRetrievePackage(name string) RetrievePackage {
	parts := strings.Split(name, "|")

	var re *regexp.Regexp = nil
	if len(parts) > 1 && parts[1] != "" {
		re, _ = regexp.Compile(parts[1])
	}

	return RetrievePackage{
		Name:              parts[0],
		allowedPreVersion: re,
	}
}

// IsMatchingPreRelease checks if the pre-release version is allowed.
func (r RetrievePackage) IsMatchingPreRelease(preRelease string) bool {
	if r.allowedPreVersion == nil {
		return false
	}
	return r.allowedPreVersion.MatchString(preRelease)
}
