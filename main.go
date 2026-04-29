package main

import (
	"path/filepath"

	"github.com/pjanzen/openproject-tracker/internal/auth"
	"github.com/pjanzen/openproject-tracker/internal/config"
	"github.com/pjanzen/openproject-tracker/internal/storage"
	"github.com/pjanzen/openproject-tracker/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}

	jar := auth.NewPersistentJar()
	dir, err := storage.ConfigDir()
	if err == nil {
		_ = jar.Load(filepath.Join(dir, "cookies.json"))
	}

	ui.Run(cfg, jar)
}
