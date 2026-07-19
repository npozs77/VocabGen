package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// ServiceAccount represents a single API-key-authenticated service account.
type ServiceAccount struct {
	Name    string `yaml:"name"`
	KeyHash string `yaml:"key_hash"`
	Scope   string `yaml:"scope"`
}

// UsersConfig holds all service accounts loaded from users.yaml.
type UsersConfig struct {
	ServiceAccounts []ServiceAccount `yaml:"service_accounts"`
}

// LoadUsersConfig reads and parses a users.yaml file at path.
// Returns nil (no error) if the file does not exist — this signals
// that auth is disabled (open access mode).
func LoadUsersConfig(path string) (*UsersConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("auth: read users config: %w", err)
	}

	var cfg UsersConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("auth: parse users config: %w", err)
	}

	// Validate entries
	for i, sa := range cfg.ServiceAccounts {
		if sa.Name == "" {
			return nil, fmt.Errorf("auth: service_accounts[%d]: name is required", i)
		}
		if sa.KeyHash == "" {
			return nil, fmt.Errorf("auth: service_accounts[%d] (%s): key_hash is required", i, sa.Name)
		}
		if sa.Scope == "" {
			cfg.ServiceAccounts[i].Scope = "read-only"
		}
	}

	return &cfg, nil
}

// ValidateAPIKey checks whether the provided plaintext key matches any
// service-account's bcrypt hash. Returns the matching ServiceAccount if
// found, or nil if no match.
func ValidateAPIKey(cfg *UsersConfig, key string) *ServiceAccount {
	if cfg == nil || len(cfg.ServiceAccounts) == 0 {
		return nil
	}
	for i := range cfg.ServiceAccounts {
		if bcrypt.CompareHashAndPassword([]byte(cfg.ServiceAccounts[i].KeyHash), []byte(key)) == nil {
			return &cfg.ServiceAccounts[i]
		}
	}
	return nil
}

// GenerateAPIKey generates a cryptographically random 32-byte hex-encoded API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate random key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashAPIKey produces a bcrypt hash of the given plaintext API key.
func HashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash key: %w", err)
	}
	return string(hash), nil
}

// ProvisionUsersConfig creates a users.yaml at the given path with a single
// service-account entry using the provided name, key hash, and scope.
// It creates parent directories as needed. Returns an error if the file
// already exists.
func ProvisionUsersConfig(path, name, keyHash, scope string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("auth: %s already exists", path)
	}

	if scope == "" {
		scope = "read-only"
	}

	cfg := UsersConfig{
		ServiceAccounts: []ServiceAccount{
			{Name: name, KeyHash: keyHash, Scope: scope},
		},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("auth: marshal users config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("auth: create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("auth: write users config: %w", err)
	}

	return nil
}
