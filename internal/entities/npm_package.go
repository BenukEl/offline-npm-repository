package entities

import (
	"regexp"
	"strings"
	"time"
)

type NpmPackage struct {
	Name         string    `json:"name"`
	Version      SemVer    `json:"version"`
	Dependencies []string  `json:"dependencies"`
	PeerDeps     []string  `json:"peerDeps"`
	Integrity    string    `json:"integrity"`
	Url          string    `json:"url"`
	ReleaseDate  time.Time `json:"releaseDate"`
}

type RetrievePackage struct {
	Name              string
	allowedPreVersion *regexp.Regexp
	fullName          string
}

func NewRetrievePackage(name string) RetrievePackage {
	parts := strings.Split(name, "|")

	var re *regexp.Regexp = nil
	if len(parts) > 1 && parts[1] != "" {
		re, _ = regexp.Compile(parts[1])
	}

	nme := parts[0]
	var fullName string
	if re == nil {
		fullName = nme
	} else {
		fullName = nme + "|" + re.String()
	}

	return RetrievePackage{
		Name:              nme,
		allowedPreVersion: re,
		fullName:          fullName,
	}
}

// IsMatchingPreRelease checks if the pre-release version is allowed.
func (r RetrievePackage) IsMatchingPreRelease(preRelease string) bool {
	if r.allowedPreVersion == nil {
		return false
	}
	return r.allowedPreVersion.MatchString(preRelease)
}

// String returns the package name with the regex.
func (r RetrievePackage) String() string {
	return r.fullName
}
