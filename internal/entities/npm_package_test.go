package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRetrievePackage(t *testing.T) {

	t.Run("The package name does not contains regex", func(t *testing.T) {
		name := "express"
		rp := NewRetrievePackage(name)

		assert.Equal(t, "express", rp.Name)
		assert.Nil(t, rp.allowedPreVersion, "AllowedPreVersion should be nil when no regex is provided")
	})

	t.Run("The package name does not contains emtpy regex", func(t *testing.T) {
		name := "express|"
		rp := NewRetrievePackage(name)

		assert.Equal(t, "express", rp.Name)
		assert.Nil(t, rp.allowedPreVersion, "AllowedPreVersion should be nil when regex is empty")
	})

	t.Run("The package name contains regex", func(t *testing.T) {
		name := "express|^4\\..*"
		rp := NewRetrievePackage(name)

		assert.Equal(t, "express", rp.Name)
		assert.NotNil(t, rp.allowedPreVersion, "AllowedPreVersion should not be nil when a regex is provided")
		assert.True(t, rp.allowedPreVersion.MatchString("4.17.1"), "Regex should match '4.17.1'")
		assert.False(t, rp.allowedPreVersion.MatchString("3.2.1"), "Regex should not match '3.2.1'")
	})

	t.Run("The package name contains invalide regex", func(t *testing.T) {
		name := "express|*invalid"
		rp := NewRetrievePackage(name)

		assert.Equal(t, "express", rp.Name)
		assert.Nil(t, rp.allowedPreVersion, "AllowedPreVersion should be nil when an invalid regex is provided")
	})

}

func TestRetrievePackage_IsMatchingPreRelease(t *testing.T) {

	t.Run("The package name does not contains regex", func(t *testing.T) {
		name := "express"
		rp := NewRetrievePackage(name)

		assert.False(t, rp.IsMatchingPreRelease("beta"))
	})

	t.Run("The package name contains regex", func(t *testing.T) {
		name := "express|beta"
		rp := NewRetrievePackage(name)

		assert.True(t, rp.IsMatchingPreRelease("beta"))
		assert.False(t, rp.IsMatchingPreRelease("rc"))
	})

}
