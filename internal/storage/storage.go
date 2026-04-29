package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ConfigDir returns ~/.config/openproject-tracker, creating it if needed.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "openproject-tracker")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// WriteJSON marshals v to JSON and writes it to path with 0600 permissions.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ReadJSON reads path and unmarshals JSON into v.
// If the file does not exist, it returns nil without modifying v.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, v)
}
