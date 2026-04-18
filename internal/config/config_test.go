package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestPropertyConfigRoundTrip verifies that for any valid Config struct,
// saving via SaveConfig then loading via LoadConfig produces an equal struct.
//
// **Validates: Requirements 34.1, 34.2**
func TestPropertyConfigRoundTrip(t *testing.T) {
	// Override configDir to a temp directory so we never touch ~/.vocabgen/
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	rapid.Check(t, func(t *rapid.T) {
		cfg := Config{
			Provider:              rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "Provider"),
			AWSProfile:            rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, "AWSProfile"),
			AWSRegion:             rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "AWSRegion"),
			ModelID:               rapid.StringMatching(`[a-zA-Z0-9_.-]*`).Draw(t, "ModelID"),
			BaseURL:               rapid.StringMatching(`[a-zA-Z0-9_:/.=-]*`).Draw(t, "BaseURL"),
			GCPProject:            rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, "GCPProject"),
			GCPRegion:             rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, "GCPRegion"),
			DefaultSourceLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "DefaultSourceLanguage"),
			DefaultTargetLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "DefaultTargetLanguage"),
			DBPath:                rapid.StringMatching(`[a-zA-Z0-9_./-]+`).Draw(t, "DBPath"),
		}

		if err := SaveConfig(cfg, ""); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		loaded, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if loaded != cfg {
			t.Fatalf("round-trip mismatch:\n  saved:  %+v\n  loaded: %+v", cfg, loaded)
		}
	})
}

// TestLoadConfigDefaultsWhenMissing verifies that LoadConfig returns
// DefaultConfig() when no config file exists on disk.
//
// Validates: Requirement 33.4
func TestLoadConfigDefaultsWhenMissing(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir() // empty dir, no config.yaml
	t.Cleanup(func() { configDir = origDir })

	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	want := DefaultConfig()
	if got != want {
		t.Fatalf("LoadConfig mismatch:\n  got:  %+v\n  want: %+v", got, want)
	}
}

// TestSaveConfigNoAPIKey verifies that the persisted YAML never contains
// an "api_key" field. API keys are runtime-only.
//
// Validates: Requirement 33.6, 33.7
func TestSaveConfigNoAPIKey(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	cfg := Config{
		Provider:              "openai",
		AWSRegion:             "eu-west-1",
		DefaultSourceLanguage: "de",
		DefaultTargetLanguage: "en",
		DBPath:                "/tmp/test.db",
	}

	if err := SaveConfig(cfg, ""); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}

	if strings.Contains(string(data), "api_key") {
		t.Fatalf("config YAML must not contain api_key, got:\n%s", data)
	}
}

// TestSaveConfigCreatesDirectory verifies that SaveConfig creates the
// config directory when it doesn't already exist.
//
// Validates: Requirement 33.5
func TestSaveConfigCreatesDirectory(t *testing.T) {
	origDir := configDir
	configDir = filepath.Join(t.TempDir(), "subdir")
	t.Cleanup(func() { configDir = origDir })

	cfg := DefaultConfig()
	if err := SaveConfig(cfg, ""); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		t.Fatalf("config file not found after SaveConfig: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected a file, got a directory")
	}
}
