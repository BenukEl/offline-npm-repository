package integrity

import (
	"crypto/sha512"
	"encoding/base64"
	"hash"
)

type IntegrityChecker interface {
	NewHash() hash.Hash
	GetSha512(hasher hash.Hash) string
}

type integrityChecker struct {
}

func NewIntegrityChecker() *integrityChecker {
	return &integrityChecker{}
}

func (i *integrityChecker) NewHash() hash.Hash {
	return sha512.New()

}

func (i *integrityChecker) GetSha512(hasher hash.Hash) string {
	return "sha512-" + base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}
