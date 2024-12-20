package entities

type NpmPackage struct {
	Name         string   `json:"name"`
	Version      SemVer   `json:"version"`
	Dependencies []string `json:"dependencies"`
	PeerDeps     []string `json:"peerDeps"`
	Integrity    string   `json:"integrity"`
	Url          string   `json:"url"`
}
