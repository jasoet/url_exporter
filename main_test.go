package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionVariables(t *testing.T) {
	// Test that version variables are initialized with default values
	assert.Equal(t, "dev", version, "Version should default to 'dev'")
	assert.Equal(t, "unknown", commit, "Commit should default to 'unknown'")
	assert.Equal(t, "unknown", date, "Date should default to 'unknown'")
	assert.Equal(t, "unknown", builtBy, "BuiltBy should default to 'unknown'")
}

func TestVersionVariablesCanBeSet(t *testing.T) {
	// Save original values
	origVersion := version
	origCommit := commit
	origDate := date
	origBuiltBy := builtBy

	// Test that version variables can be set (simulating build-time injection)
	version = "v1.0.0"
	commit = "abc123"
	date = "2024-01-01"
	builtBy = "goreleaser"

	assert.Equal(t, "v1.0.0", version)
	assert.Equal(t, "abc123", commit)
	assert.Equal(t, "2024-01-01", date)
	assert.Equal(t, "goreleaser", builtBy)

	// Restore original values
	version = origVersion
	commit = origCommit
	date = origDate
	builtBy = origBuiltBy
}