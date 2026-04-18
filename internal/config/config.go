package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// configDir is the directory where the config file is stored.
// Tests can override this to use a temp directory.
var configDir = ""

// SetConfigDirForTest overrides the config directory. Call with "" to reset.
// Only use in tests.
func SetConfigDirForTest(dir string) {
	configDir = dir
}

// ProfileConfig holds provider-related fields for a named profile.
type ProfileConfig struct {
	Provider   string `yaml:"provider"`
	AWSProfile string `yaml:"aws_profile,omitempty"`
	AWSRegion  string `yaml:"aws_region,omitempty"`
	ModelID    string `yaml:"model_id,omitempty"`
	BaseURL    string `yaml:"base_url,omitempty"`
	GCPProject string `yaml:"gcp_project,omitempty"`
	GCPRegion  string `yaml:"gcp_region,omitempty"`
}

// FileConfig represents the multi-profile YAML structure.
type FileConfig struct {
	DefaultProfile        string                   `yaml:"default_profile,omitempty"`
	Profiles              map[string]ProfileConfig `yaml:"profiles,omitempty"`
	DefaultSourceLanguage string                   `yaml:"default_source_language"`
	DefaultTargetLanguage string                   `yaml:"default_target_language"`
	DBPath                string                   `yaml:"db_path"`
}

// Config holds application settings persisted to ~/.vocabgen/config.yaml.
// API keys are deliberately excluded — they come from env vars or CLI flags.
type Config struct {
	Provider              string `yaml:"provider"`
	AWSProfile            string `yaml:"aws_profile,omitempty"`
	AWSRegion             string `yaml:"aws_region"`
	ModelID               string `yaml:"model_id,omitempty"`
	BaseURL               string `yaml:"base_url,omitempty"`
	GCPProject            string `yaml:"gcp_project,omitempty"`
	GCPRegion             string `yaml:"gcp_region,omitempty"`
	DefaultSourceLanguage string `yaml:"default_source_language"`
	DefaultTargetLanguage string `yaml:"default_target_language"`
	DBPath                string `yaml:"db_path"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Provider:              "bedrock",
		AWSRegion:             "us-east-1",
		DefaultSourceLanguage: "nl",
		DefaultTargetLanguage: "hu",
		DBPath:                "~/.vocabgen/vocabgen.db",
	}
}

// getConfigDir returns the resolved config directory path.
func getConfigDir() (string, error) {
	if configDir != "" {
		return configDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve home directory: %w", err)
	}
	return filepath.Join(home, ".vocabgen"), nil
}

// FilePath returns the resolved path to the config file.
func FilePath() string {
	dir, err := getConfigDir()
	if err != nil {
		return "~/.vocabgen/config.yaml"
	}
	return filepath.Join(dir, "config.yaml")
}

// isMultiProfile checks whether raw YAML data contains a profiles: key.
func isMultiProfile(data []byte) bool {
	var probe struct {
		Profiles map[string]any `yaml:"profiles"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return false
	}
	return len(probe.Profiles) > 0
}

// readConfigFile reads the raw YAML bytes from the config file.
// Returns nil, nil when the file does not exist.
func readConfigFile() ([]byte, error) {
	dir, err := getConfigDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("config: read file: %w", err)
	}
	return data, nil
}

// resolveProfile populates a flat Config from a FileConfig and a named profile.
func resolveProfile(fc FileConfig, name string) (Config, error) {
	p, ok := fc.Profiles[name]
	if !ok {
		available := make([]string, 0, len(fc.Profiles))
		for k := range fc.Profiles {
			available = append(available, k)
		}
		return Config{}, fmt.Errorf("config: profile %q not found; available profiles: %v", name, available)
	}
	cfg := Config{
		Provider:              p.Provider,
		AWSProfile:            p.AWSProfile,
		AWSRegion:             p.AWSRegion,
		ModelID:               p.ModelID,
		BaseURL:               p.BaseURL,
		GCPProject:            p.GCPProject,
		GCPRegion:             p.GCPRegion,
		DefaultSourceLanguage: fc.DefaultSourceLanguage,
		DefaultTargetLanguage: fc.DefaultTargetLanguage,
		DBPath:                fc.DBPath,
	}
	return cfg, nil
}

// applyDefaults fills in zero-value fields from DefaultConfig.
func applyDefaults(cfg *Config) {
	defaults := DefaultConfig()
	if cfg.Provider == "" {
		cfg.Provider = defaults.Provider
	}
	if cfg.AWSRegion == "" {
		cfg.AWSRegion = defaults.AWSRegion
	}
	if cfg.DefaultSourceLanguage == "" {
		cfg.DefaultSourceLanguage = defaults.DefaultSourceLanguage
	}
	if cfg.DefaultTargetLanguage == "" {
		cfg.DefaultTargetLanguage = defaults.DefaultTargetLanguage
	}
	if cfg.DBPath == "" {
		cfg.DBPath = defaults.DBPath
	}
}

// LoadConfig reads config from ~/.vocabgen/config.yaml.
// Returns DefaultConfig() if the file doesn't exist.
// Supports both flat format (backward compatible) and multi-profile format.
func LoadConfig() (Config, error) {
	data, err := readConfigFile()
	if err != nil {
		return DefaultConfig(), err
	}
	if data == nil {
		return DefaultConfig(), nil
	}

	if isMultiProfile(data) {
		var fc FileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return DefaultConfig(), fmt.Errorf("config: parse yaml: %w", err)
		}
		profileName := fc.DefaultProfile
		if profileName == "" {
			// Use first profile if default_profile is unset.
			for k := range fc.Profiles {
				profileName = k
				break
			}
		}
		if profileName == "" {
			return DefaultConfig(), fmt.Errorf("config: multi-profile format but no profiles defined")
		}
		cfg, err := resolveProfile(fc, profileName)
		if err != nil {
			return DefaultConfig(), err
		}
		applyDefaults(&cfg)
		return cfg, nil
	}

	// Flat format (existing behavior).
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("config: parse yaml: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

// LoadConfigWithProfile loads config and resolves the named profile.
// For flat format, only "default" is accepted as profile name.
// Returns a descriptive error listing available profiles if name not found.
func LoadConfigWithProfile(profileName string) (Config, error) {
	data, err := readConfigFile()
	if err != nil {
		return DefaultConfig(), err
	}
	if data == nil {
		if profileName == "default" {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("config: profile %q not found; available profiles: [default]", profileName)
	}

	if isMultiProfile(data) {
		var fc FileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return DefaultConfig(), fmt.Errorf("config: parse yaml: %w", err)
		}
		cfg, err := resolveProfile(fc, profileName)
		if err != nil {
			return Config{}, err
		}
		applyDefaults(&cfg)
		return cfg, nil
	}

	// Flat format — only accept "default" as profile name.
	if profileName != "default" {
		return Config{}, fmt.Errorf("config: profile %q not found; available profiles: [default]", profileName)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("config: parse yaml: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

// ListProfiles returns available profile names and the default profile name.
// For flat format, returns ["default"] with default "default".
func ListProfiles() (profiles []string, defaultProfile string, err error) {
	data, err := readConfigFile()
	if err != nil {
		return nil, "", err
	}
	if data == nil {
		return []string{"default"}, "default", nil
	}

	if isMultiProfile(data) {
		var fc FileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, "", fmt.Errorf("config: parse yaml: %w", err)
		}
		names := make([]string, 0, len(fc.Profiles))
		for k := range fc.Profiles {
			names = append(names, k)
		}
		def := fc.DefaultProfile
		if def == "" && len(names) > 0 {
			def = names[0]
		}
		return names, def, nil
	}

	return []string{"default"}, "default", nil
}

// SaveConfig writes config to ~/.vocabgen/config.yaml.
// Creates the directory if it doesn't exist.
// Never writes API keys to the file.
// If the existing config file uses multi-profile format, SaveConfig preserves
// that structure by updating the specified activeProfile within the FileConfig.
// If activeProfile is empty, it falls back to the file's default_profile.
func SaveConfig(cfg Config, activeProfile string) error {
	dir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: create directory: %w", err)
	}

	// Check if existing file is multi-profile and preserve structure.
	existing, _ := readConfigFile()
	if existing != nil && isMultiProfile(existing) {
		var fc FileConfig
		if err := yaml.Unmarshal(existing, &fc); err == nil {
			profileName := activeProfile
			if profileName == "" {
				profileName = fc.DefaultProfile
			}
			if profileName == "" {
				for k := range fc.Profiles {
					profileName = k
					break
				}
			}
			if profileName != "" {
				fc.Profiles[profileName] = ProfileConfig{
					Provider:   cfg.Provider,
					AWSProfile: cfg.AWSProfile,
					AWSRegion:  cfg.AWSRegion,
					ModelID:    cfg.ModelID,
					BaseURL:    cfg.BaseURL,
					GCPProject: cfg.GCPProject,
					GCPRegion:  cfg.GCPRegion,
				}
				fc.DefaultSourceLanguage = cfg.DefaultSourceLanguage
				fc.DefaultTargetLanguage = cfg.DefaultTargetLanguage
				fc.DBPath = cfg.DBPath
				return SaveFileConfig(fc)
			}
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal yaml: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("config: write file: %w", err)
	}
	return nil
}

// SaveFileConfig writes a multi-profile FileConfig to ~/.vocabgen/config.yaml.
// Creates the directory if it doesn't exist.
func SaveFileConfig(fc FileConfig) error {
	dir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: create directory: %w", err)
	}

	data, err := yaml.Marshal(fc)
	if err != nil {
		return fmt.Errorf("config: marshal yaml: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("config: write file: %w", err)
	}
	return nil
}

// CreateProfile creates a new named profile by copying values from an existing profile.
// If the config file uses flat format, it is converted to multi-profile format first.
// The new profile becomes the default. Returns an error if newName already exists
// or sourceProfile is not found.
func CreateProfile(newName, sourceProfile string) error {
	data, err := readConfigFile()
	if err != nil {
		return err
	}

	var fc FileConfig

	if data != nil && isMultiProfile(data) {
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return fmt.Errorf("config: parse yaml: %w", err)
		}
	} else {
		// Flat format or no file — convert to multi-profile with implicit "default" profile.
		var flat Config
		if data != nil {
			if err := yaml.Unmarshal(data, &flat); err != nil {
				return fmt.Errorf("config: parse yaml: %w", err)
			}
		} else {
			flat = DefaultConfig()
		}
		applyDefaults(&flat)
		fc = FileConfig{
			DefaultProfile: "default",
			Profiles: map[string]ProfileConfig{
				"default": {
					Provider:   flat.Provider,
					AWSProfile: flat.AWSProfile,
					AWSRegion:  flat.AWSRegion,
					ModelID:    flat.ModelID,
					BaseURL:    flat.BaseURL,
					GCPProject: flat.GCPProject,
					GCPRegion:  flat.GCPRegion,
				},
			},
			DefaultSourceLanguage: flat.DefaultSourceLanguage,
			DefaultTargetLanguage: flat.DefaultTargetLanguage,
			DBPath:                flat.DBPath,
		}
	}

	// Check for duplicate name.
	if _, exists := fc.Profiles[newName]; exists {
		return fmt.Errorf("config: profile %q already exists", newName)
	}

	// Look up source profile.
	src, ok := fc.Profiles[sourceProfile]
	if !ok {
		available := make([]string, 0, len(fc.Profiles))
		for k := range fc.Profiles {
			available = append(available, k)
		}
		return fmt.Errorf("config: source profile %q not found; available profiles: %v", sourceProfile, available)
	}

	// Copy source values into new profile, but clear ModelID so the user
	// is prompted to set a provider-appropriate model for the new profile.
	fc.Profiles[newName] = ProfileConfig{
		Provider:   src.Provider,
		AWSProfile: src.AWSProfile,
		AWSRegion:  src.AWSRegion,
		ModelID:    "",
		BaseURL:    src.BaseURL,
		GCPProject: src.GCPProject,
		GCPRegion:  src.GCPRegion,
	}
	fc.DefaultProfile = newName

	return SaveFileConfig(fc)
}
