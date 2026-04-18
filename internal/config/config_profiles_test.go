package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// genProfileConfig generates a random ProfileConfig for property tests.
func genProfileConfig(t *rapid.T, label string) ProfileConfig {
	return ProfileConfig{
		Provider:   rapid.SampledFrom([]string{"bedrock", "openai", "anthropic", "vertexai"}).Draw(t, label+".Provider"),
		AWSProfile: rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, label+".AWSProfile"),
		AWSRegion:  rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, label+".AWSRegion"),
		ModelID:    rapid.StringMatching(`[a-zA-Z0-9_.-]*`).Draw(t, label+".ModelID"),
		BaseURL:    rapid.StringMatching(`[a-zA-Z0-9_:/.=-]*`).Draw(t, label+".BaseURL"),
		GCPProject: rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, label+".GCPProject"),
		GCPRegion:  rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, label+".GCPRegion"),
	}
}

// TestPropertyP20_MultiProfileRoundTrip verifies that for any valid FileConfig
// with 1-5 profiles, saving via SaveFileConfig then loading via LoadConfigWithProfile
// produces matching values for each profile.
//
// **Property 20: Multi-profile round-trip**
// **Validates: Requirements 58.1, 58.6**
func TestPropertyP20_MultiProfileRoundTrip(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	rapid.Check(t, func(t *rapid.T) {
		numProfiles := rapid.IntRange(1, 5).Draw(t, "numProfiles")
		profiles := make(map[string]ProfileConfig, numProfiles)
		var names []string
		for i := 0; i < numProfiles; i++ {
			name := rapid.StringMatching(`[a-z][a-z0-9_-]{0,9}`).Draw(t, "name")
			// Ensure unique names
			for _, exists := profiles[name]; exists; _, exists = profiles[name] {
				name = name + rapid.StringMatching(`[a-z]`).Draw(t, "suffix")
			}
			profiles[name] = genProfileConfig(t, name)
			names = append(names, name)
		}

		defaultProfile := names[rapid.IntRange(0, len(names)-1).Draw(t, "defaultIdx")]

		fc := FileConfig{
			DefaultProfile:        defaultProfile,
			Profiles:              profiles,
			DefaultSourceLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "srcLang"),
			DefaultTargetLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "tgtLang"),
			DBPath:                rapid.StringMatching(`[a-zA-Z0-9_./-]+`).Draw(t, "dbPath"),
		}

		if err := SaveFileConfig(fc); err != nil {
			t.Fatalf("SaveFileConfig failed: %v", err)
		}

		// Verify each profile round-trips correctly.
		for name, wantProfile := range fc.Profiles {
			got, err := LoadConfigWithProfile(name)
			if err != nil {
				t.Fatalf("LoadConfigWithProfile(%q) failed: %v", name, err)
			}
			if got.Provider != wantProfile.Provider {
				t.Fatalf("profile %q: Provider mismatch: got %q, want %q", name, got.Provider, wantProfile.Provider)
			}
			if got.AWSProfile != wantProfile.AWSProfile {
				t.Fatalf("profile %q: AWSProfile mismatch: got %q, want %q", name, got.AWSProfile, wantProfile.AWSProfile)
			}
			if got.ModelID != wantProfile.ModelID {
				t.Fatalf("profile %q: ModelID mismatch: got %q, want %q", name, got.ModelID, wantProfile.ModelID)
			}
			if got.BaseURL != wantProfile.BaseURL {
				t.Fatalf("profile %q: BaseURL mismatch: got %q, want %q", name, got.BaseURL, wantProfile.BaseURL)
			}
			if got.DefaultSourceLanguage != fc.DefaultSourceLanguage {
				t.Fatalf("profile %q: DefaultSourceLanguage mismatch: got %q, want %q", name, got.DefaultSourceLanguage, fc.DefaultSourceLanguage)
			}
			if got.DefaultTargetLanguage != fc.DefaultTargetLanguage {
				t.Fatalf("profile %q: DefaultTargetLanguage mismatch: got %q, want %q", name, got.DefaultTargetLanguage, fc.DefaultTargetLanguage)
			}
			if got.DBPath != fc.DBPath {
				t.Fatalf("profile %q: DBPath mismatch: got %q, want %q", name, got.DBPath, fc.DBPath)
			}
		}

		// Verify default profile is loaded by LoadConfig.
		defaultCfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}
		wantDefault := fc.Profiles[defaultProfile]
		if defaultCfg.Provider != wantDefault.Provider {
			t.Fatalf("LoadConfig default profile: Provider mismatch: got %q, want %q", defaultCfg.Provider, wantDefault.Provider)
		}
	})
}

// TestPropertyP21_UnknownProfileReturnsError verifies that requesting a
// profile name not present in the profiles map always returns an error.
//
// **Property 21: Unknown profile name returns error**
// **Validates: Requirement 58.4**
func TestPropertyP21_UnknownProfileReturnsError(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	rapid.Check(t, func(t *rapid.T) {
		// Create a FileConfig with known profiles.
		fc := FileConfig{
			DefaultProfile: "prod",
			Profiles: map[string]ProfileConfig{
				"prod":  {Provider: "bedrock", AWSRegion: "us-east-1"},
				"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1"},
			},
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}
		if err := SaveFileConfig(fc); err != nil {
			t.Fatalf("SaveFileConfig failed: %v", err)
		}

		// Generate a random name that is NOT in the profiles map.
		badName := rapid.StringMatching(`[a-z][a-z0-9]{2,10}`).Draw(t, "badName")
		for badName == "prod" || badName == "local" {
			badName = badName + "x"
		}

		_, err := LoadConfigWithProfile(badName)
		if err == nil {
			t.Fatalf("expected error for unknown profile %q, got nil", badName)
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error should mention 'not found', got: %v", err)
		}
		if !strings.Contains(err.Error(), "available profiles") {
			t.Fatalf("error should list available profiles, got: %v", err)
		}
	})
}

// TestPropertyP22_FlatConfigBackwardCompat verifies that a flat config file
// (no profiles: key) loads correctly and is accessible as the "default" profile.
//
// **Property 22: Flat config backward compatibility**
// **Validates: Requirement 58.5**
func TestPropertyP22_FlatConfigBackwardCompat(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	rapid.Check(t, func(t *rapid.T) {
		cfg := Config{
			Provider:              rapid.SampledFrom([]string{"bedrock", "openai", "anthropic"}).Draw(t, "Provider"),
			AWSProfile:            rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, "AWSProfile"),
			AWSRegion:             rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "AWSRegion"),
			ModelID:               rapid.StringMatching(`[a-zA-Z0-9_.-]*`).Draw(t, "ModelID"),
			BaseURL:               rapid.StringMatching(`[a-zA-Z0-9_:/.=-]*`).Draw(t, "BaseURL"),
			GCPProject:            rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, "GCPProject"),
			GCPRegion:             rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(t, "GCPRegion"),
			DefaultSourceLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "SrcLang"),
			DefaultTargetLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(t, "TgtLang"),
			DBPath:                rapid.StringMatching(`[a-zA-Z0-9_./-]+`).Draw(t, "DBPath"),
		}

		// Save in flat format (direct YAML marshal, bypassing SaveConfig's multi-profile detection).
		dir, err := getConfigDir()
		if err != nil {
			t.Fatalf("getConfigDir: %v", err)
		}
		_ = os.MkdirAll(dir, 0o755)
		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		// LoadConfig should work.
		loaded, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}
		if loaded != cfg {
			t.Fatalf("LoadConfig mismatch:\n  got:  %+v\n  want: %+v", loaded, cfg)
		}

		// LoadConfigWithProfile("default") should work.
		loadedProfile, err := LoadConfigWithProfile("default")
		if err != nil {
			t.Fatalf("LoadConfigWithProfile(default) failed: %v", err)
		}
		if loadedProfile != cfg {
			t.Fatalf("LoadConfigWithProfile(default) mismatch:\n  got:  %+v\n  want: %+v", loadedProfile, cfg)
		}

		// LoadConfigWithProfile("other") should fail.
		_, err = LoadConfigWithProfile("other")
		if err == nil {
			t.Fatal("expected error for non-default profile on flat config")
		}

		// ListProfiles should return ["default"].
		profiles, def, err := ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles failed: %v", err)
		}
		if def != "default" {
			t.Fatalf("expected default profile 'default', got %q", def)
		}
		if len(profiles) != 1 || profiles[0] != "default" {
			t.Fatalf("expected [default], got %v", profiles)
		}
	})
}

// TestLoadConfigWithProfile_TableDriven tests specific profile resolution scenarios.
//
// Validates: Requirements 58.3, 58.4
func TestLoadConfigWithProfile_TableDriven(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	// Write a multi-profile config.
	fc := FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]ProfileConfig{
			"prod": {
				Provider:  "bedrock",
				AWSRegion: "us-east-1",
				ModelID:   "us.anthropic.claude-sonnet-4-20250514-v1:0",
			},
			"sandbox": {
				Provider:  "bedrock",
				AWSRegion: "eu-west-1",
				ModelID:   "anthropic.claude-3-haiku-20240307-v1:0",
			},
			"local": {
				Provider: "openai",
				BaseURL:  "http://localhost:11434/v1",
				ModelID:  "translategemma",
			},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	tests := []struct {
		name         string
		profile      string
		wantErr      bool
		wantErrSub   string
		wantProvider string
		wantRegion   string
		wantModelID  string
		wantBaseURL  string
	}{
		{
			name:         "sandbox profile",
			profile:      "sandbox",
			wantProvider: "bedrock",
			wantRegion:   "eu-west-1",
			wantModelID:  "anthropic.claude-3-haiku-20240307-v1:0",
		},
		{
			name:         "local profile",
			profile:      "local",
			wantProvider: "openai",
			wantBaseURL:  "http://localhost:11434/v1",
			wantModelID:  "translategemma",
		},
		{
			name:         "prod profile",
			profile:      "prod",
			wantProvider: "bedrock",
			wantRegion:   "us-east-1",
			wantModelID:  "us.anthropic.claude-sonnet-4-20250514-v1:0",
		},
		{
			name:       "nonexistent profile",
			profile:    "nonexistent",
			wantErr:    true,
			wantErrSub: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfigWithProfile(tc.profile)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrSub != "" && !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got: %v", tc.wantErrSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Provider != tc.wantProvider {
				t.Fatalf("Provider: got %q, want %q", cfg.Provider, tc.wantProvider)
			}
			if tc.wantRegion != "" && cfg.AWSRegion != tc.wantRegion {
				t.Fatalf("AWSRegion: got %q, want %q", cfg.AWSRegion, tc.wantRegion)
			}
			if tc.wantModelID != "" && cfg.ModelID != tc.wantModelID {
				t.Fatalf("ModelID: got %q, want %q", cfg.ModelID, tc.wantModelID)
			}
			if tc.wantBaseURL != "" && cfg.BaseURL != tc.wantBaseURL {
				t.Fatalf("BaseURL: got %q, want %q", cfg.BaseURL, tc.wantBaseURL)
			}
			// Shared fields should come from FileConfig.
			if cfg.DefaultSourceLanguage != "nl" {
				t.Fatalf("DefaultSourceLanguage: got %q, want 'nl'", cfg.DefaultSourceLanguage)
			}
			if cfg.DefaultTargetLanguage != "hu" {
				t.Fatalf("DefaultTargetLanguage: got %q, want 'hu'", cfg.DefaultTargetLanguage)
			}
		})
	}
}

// TestListProfiles_TableDriven tests ListProfiles for both flat and multi-profile formats.
//
// Validates: Requirement 58.7
func TestListProfiles_TableDriven(t *testing.T) {
	origDir := configDir
	t.Cleanup(func() { configDir = origDir })

	t.Run("no config file", func(t *testing.T) {
		configDir = t.TempDir()
		profiles, def, err := ListProfiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if def != "default" {
			t.Fatalf("expected default 'default', got %q", def)
		}
		if len(profiles) != 1 || profiles[0] != "default" {
			t.Fatalf("expected [default], got %v", profiles)
		}
	})

	t.Run("flat config", func(t *testing.T) {
		configDir = t.TempDir()
		if err := SaveConfig(DefaultConfig(), ""); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}
		profiles, def, err := ListProfiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if def != "default" {
			t.Fatalf("expected default 'default', got %q", def)
		}
		if len(profiles) != 1 || profiles[0] != "default" {
			t.Fatalf("expected [default], got %v", profiles)
		}
	})

	t.Run("multi-profile config", func(t *testing.T) {
		configDir = t.TempDir()
		fc := FileConfig{
			DefaultProfile: "prod",
			Profiles: map[string]ProfileConfig{
				"prod":    {Provider: "bedrock"},
				"local":   {Provider: "openai"},
				"sandbox": {Provider: "bedrock"},
			},
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}
		if err := SaveFileConfig(fc); err != nil {
			t.Fatalf("SaveFileConfig: %v", err)
		}
		profiles, def, err := ListProfiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if def != "prod" {
			t.Fatalf("expected default 'prod', got %q", def)
		}
		sort.Strings(profiles)
		want := []string{"local", "prod", "sandbox"}
		if len(profiles) != len(want) {
			t.Fatalf("expected %v, got %v", want, profiles)
		}
		for i, p := range profiles {
			if p != want[i] {
				t.Fatalf("expected %v, got %v", want, profiles)
			}
		}
	})
}

// TestSaveConfigPreservesMultiProfile verifies that SaveConfig on a multi-profile
// file updates the default profile without flattening the structure.
//
// Validates: Requirement 58.6
func TestSaveConfigPreservesMultiProfile(t *testing.T) {
	origDir := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = origDir })

	// Write initial multi-profile config.
	fc := FileConfig{
		DefaultProfile: "prod",
		Profiles: map[string]ProfileConfig{
			"prod":  {Provider: "bedrock", AWSRegion: "us-east-1"},
			"local": {Provider: "openai", BaseURL: "http://localhost:11434/v1"},
		},
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := SaveFileConfig(fc); err != nil {
		t.Fatalf("SaveFileConfig: %v", err)
	}

	// Update via SaveConfig (simulates Web UI save).
	updated := Config{
		Provider:              "anthropic",
		AWSRegion:             "eu-west-1",
		ModelID:               "claude-sonnet-4-20250514",
		DefaultSourceLanguage: "de",
		DefaultTargetLanguage: "en",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
	if err := SaveConfig(updated, "prod"); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Verify the file is still multi-profile.
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "profiles:") {
		t.Fatalf("expected multi-profile format preserved, got:\n%s", data)
	}

	// Verify the "local" profile is untouched.
	localCfg, err := LoadConfigWithProfile("local")
	if err != nil {
		t.Fatalf("LoadConfigWithProfile(local): %v", err)
	}
	if localCfg.Provider != "openai" {
		t.Fatalf("local profile provider should be 'openai', got %q", localCfg.Provider)
	}
	if localCfg.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("local profile base_url should be preserved, got %q", localCfg.BaseURL)
	}

	// Verify the "prod" profile was updated.
	prodCfg, err := LoadConfigWithProfile("prod")
	if err != nil {
		t.Fatalf("LoadConfigWithProfile(prod): %v", err)
	}
	if prodCfg.Provider != "anthropic" {
		t.Fatalf("prod profile provider should be 'anthropic', got %q", prodCfg.Provider)
	}
	if prodCfg.ModelID != "claude-sonnet-4-20250514" {
		t.Fatalf("prod profile model_id should be updated, got %q", prodCfg.ModelID)
	}

	// Verify shared fields were updated.
	if prodCfg.DefaultSourceLanguage != "de" {
		t.Fatalf("DefaultSourceLanguage should be 'de', got %q", prodCfg.DefaultSourceLanguage)
	}
}

// TestCreateProfile_TableDriven tests CreateProfile for various scenarios.
//
// Validates: Requirements 58.11, 58.12
func TestCreateProfile_TableDriven(t *testing.T) {
	origDir := configDir
	t.Cleanup(func() { configDir = origDir })

	t.Run("flat config creates multi-profile with new profile", func(t *testing.T) {
		configDir = t.TempDir()
		// Save a flat config.
		cfg := Config{
			Provider:              "bedrock",
			AWSRegion:             "us-east-1",
			ModelID:               "claude-v1",
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		dir, _ := getConfigDir()
		_ = os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		if err := CreateProfile("sandbox", "default"); err != nil {
			t.Fatalf("CreateProfile: %v", err)
		}

		// Should now be multi-profile.
		profiles, def, err := ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles: %v", err)
		}
		if def != "sandbox" {
			t.Fatalf("expected default 'sandbox', got %q", def)
		}
		sort.Strings(profiles)
		if len(profiles) != 2 {
			t.Fatalf("expected 2 profiles, got %v", profiles)
		}

		// New profile should have same provider values as source.
		sandboxCfg, err := LoadConfigWithProfile("sandbox")
		if err != nil {
			t.Fatalf("LoadConfigWithProfile(sandbox): %v", err)
		}
		if sandboxCfg.Provider != "bedrock" {
			t.Fatalf("expected provider 'bedrock', got %q", sandboxCfg.Provider)
		}
		if sandboxCfg.AWSRegion != "us-east-1" {
			t.Fatalf("expected region 'us-east-1', got %q", sandboxCfg.AWSRegion)
		}
	})

	t.Run("duplicate name returns error", func(t *testing.T) {
		configDir = t.TempDir()
		fc := FileConfig{
			DefaultProfile: "prod",
			Profiles: map[string]ProfileConfig{
				"prod": {Provider: "bedrock", AWSRegion: "us-east-1"},
			},
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}
		if err := SaveFileConfig(fc); err != nil {
			t.Fatalf("SaveFileConfig: %v", err)
		}

		err := CreateProfile("prod", "prod")
		if err == nil {
			t.Fatal("expected error for duplicate name, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("expected 'already exists' in error, got: %v", err)
		}
	})

	t.Run("nonexistent source returns error", func(t *testing.T) {
		configDir = t.TempDir()
		fc := FileConfig{
			DefaultProfile: "prod",
			Profiles: map[string]ProfileConfig{
				"prod": {Provider: "bedrock"},
			},
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}
		if err := SaveFileConfig(fc); err != nil {
			t.Fatalf("SaveFileConfig: %v", err)
		}

		err := CreateProfile("new", "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent source, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected 'not found' in error, got: %v", err)
		}
	})

	t.Run("no config file creates from defaults", func(t *testing.T) {
		configDir = t.TempDir()

		if err := CreateProfile("sandbox", "default"); err != nil {
			t.Fatalf("CreateProfile: %v", err)
		}

		profiles, def, err := ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles: %v", err)
		}
		if def != "sandbox" {
			t.Fatalf("expected default 'sandbox', got %q", def)
		}
		if len(profiles) != 2 {
			t.Fatalf("expected 2 profiles, got %v", profiles)
		}
	})
}

// TestPropertyP24_DuplicateNameNeverModifiesProfiles verifies that calling
// CreateProfile with a name that already exists always returns an error
// and never modifies the existing profiles.
//
// **Property 24: Duplicate name always returns error without modifying profiles**
// **Validates: Requirement 58.12**
func TestPropertyP24_DuplicateNameNeverModifiesProfiles(t *testing.T) {
	origDir := configDir
	t.Cleanup(func() { configDir = origDir })

	rapid.Check(t, func(rt *rapid.T) {
		configDir = t.TempDir()

		numProfiles := rapid.IntRange(1, 4).Draw(rt, "numProfiles")
		profiles := make(map[string]ProfileConfig, numProfiles)
		var names []string
		for i := 0; i < numProfiles; i++ {
			name := rapid.StringMatching(`[a-z][a-z0-9]{1,6}`).Draw(rt, "name")
			for _, exists := profiles[name]; exists; _, exists = profiles[name] {
				name = name + rapid.StringMatching(`[a-z]`).Draw(rt, "suffix")
			}
			profiles[name] = genProfileConfig(rt, name)
			names = append(names, name)
		}

		fc := FileConfig{
			DefaultProfile:        names[0],
			Profiles:              profiles,
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}
		if err := SaveFileConfig(fc); err != nil {
			rt.Fatalf("SaveFileConfig: %v", err)
		}

		// Pick an existing name to duplicate.
		dupName := names[rapid.IntRange(0, len(names)-1).Draw(rt, "dupIdx")]
		sourceName := names[rapid.IntRange(0, len(names)-1).Draw(rt, "srcIdx")]

		err := CreateProfile(dupName, sourceName)
		if err == nil {
			rt.Fatalf("expected error for duplicate name %q, got nil", dupName)
		}
		if !strings.Contains(err.Error(), "already exists") {
			rt.Fatalf("expected 'already exists' in error, got: %v", err)
		}

		// Verify profiles are unchanged.
		afterProfiles, _, err := ListProfiles()
		if err != nil {
			rt.Fatalf("ListProfiles: %v", err)
		}
		sort.Strings(afterProfiles)
		sort.Strings(names)
		if len(afterProfiles) != len(names) {
			rt.Fatalf("profile count changed: before %v, after %v", names, afterProfiles)
		}
		for i, p := range afterProfiles {
			if p != names[i] {
				rt.Fatalf("profiles changed: before %v, after %v", names, afterProfiles)
			}
		}
	})
}

// TestPropertyP25_NewProfileCopiesSourceValues verifies that a newly created
// profile always contains the same provider values as the source profile.
//
// **Property 25: New profile contains same values as source profile**
// **Validates: Requirement 58.11**
func TestPropertyP25_NewProfileCopiesSourceValues(t *testing.T) {
	origDir := configDir
	t.Cleanup(func() { configDir = origDir })

	rapid.Check(t, func(rt *rapid.T) {
		configDir = t.TempDir()

		srcProfile := genProfileConfig(rt, "src")
		fc := FileConfig{
			DefaultProfile: "source",
			Profiles: map[string]ProfileConfig{
				"source": srcProfile,
			},
			DefaultSourceLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(rt, "srcLang"),
			DefaultTargetLanguage: rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(rt, "tgtLang"),
			DBPath:                rapid.StringMatching(`[a-zA-Z0-9_./-]+`).Draw(rt, "dbPath"),
		}
		if err := SaveFileConfig(fc); err != nil {
			rt.Fatalf("SaveFileConfig: %v", err)
		}

		newName := rapid.StringMatching(`[a-z][a-z0-9]{1,6}`).Draw(rt, "newName")
		for newName == "source" {
			newName = newName + "x"
		}

		if err := CreateProfile(newName, "source"); err != nil {
			rt.Fatalf("CreateProfile: %v", err)
		}

		newCfg, err := LoadConfigWithProfile(newName)
		if err != nil {
			rt.Fatalf("LoadConfigWithProfile(%q): %v", newName, err)
		}

		// After loading, applyDefaults fills empty fields. Account for that.
		defaults := DefaultConfig()
		wantProvider := srcProfile.Provider
		if wantProvider == "" {
			wantProvider = defaults.Provider
		}
		wantRegion := srcProfile.AWSRegion
		if wantRegion == "" {
			wantRegion = defaults.AWSRegion
		}

		if newCfg.Provider != wantProvider {
			rt.Fatalf("Provider mismatch: got %q, want %q", newCfg.Provider, wantProvider)
		}
		if newCfg.AWSProfile != srcProfile.AWSProfile {
			rt.Fatalf("AWSProfile mismatch: got %q, want %q", newCfg.AWSProfile, srcProfile.AWSProfile)
		}
		if newCfg.AWSRegion != wantRegion {
			rt.Fatalf("AWSRegion mismatch: got %q, want %q", newCfg.AWSRegion, wantRegion)
		}
		if newCfg.ModelID != "" {
			rt.Fatalf("ModelID should be empty on new profile, got %q", newCfg.ModelID)
		}
		if newCfg.BaseURL != srcProfile.BaseURL {
			rt.Fatalf("BaseURL mismatch: got %q, want %q", newCfg.BaseURL, srcProfile.BaseURL)
		}
		if newCfg.GCPProject != srcProfile.GCPProject {
			rt.Fatalf("GCPProject mismatch: got %q, want %q", newCfg.GCPProject, srcProfile.GCPProject)
		}
		if newCfg.GCPRegion != srcProfile.GCPRegion {
			rt.Fatalf("GCPRegion mismatch: got %q, want %q", newCfg.GCPRegion, srcProfile.GCPRegion)
		}
	})
}
