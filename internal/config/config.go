// Package config manages application settings persisted to ~/.vocabgen/config.yaml.
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

// LoadConfig reads config from ~/.vocabgen/config.yaml.
// Returns DefaultConfig() if the file doesn't exist.
func LoadConfig() (Config, error) {
	dir, err := getConfigDir()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), fmt.Errorf("config: read file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("config: parse yaml: %w", err)
	}

	// Apply defaults for any fields not present in the config file.
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

	return cfg, nil
}

// SaveConfig writes config to ~/.vocabgen/config.yaml.
// Creates the directory if it doesn't exist.
// Never writes API keys to the file.
func SaveConfig(cfg Config) error {
	dir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: create directory: %w", err)
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
