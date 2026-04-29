package config

import (
	"path/filepath"

	"github.com/pjanzen/openproject-tracker/internal/storage"
)

// Config holds application configuration.
type Config struct {
	BaseURL       string            `json:"base_url"`
	SkipTLSVerify bool              `json:"skip_tls_verify"`
	CAPath        string            `json:"ca_path,omitempty"`
	ExtraHeaders  map[string]string `json:"extra_headers,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{BaseURL: "https://projects.unified.services"}
}

// Load reads config.json from the config directory.
func Load() (*Config, error) {
	dir, err := storage.ConfigDir()
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := storage.ReadJSON(filepath.Join(dir, "config.json"), cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the config to config.json in the config directory.
func (c *Config) Save() error {
	dir, err := storage.ConfigDir()
	if err != nil {
		return err
	}
	return storage.WriteJSON(filepath.Join(dir, "config.json"), c)
}
