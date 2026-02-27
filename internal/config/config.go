package config

import (
	"os"
	"path/filepath"
)

const (
	AppName = "gitm"
	DBName  = "gitm.db"
)

// Config holds the application configuration.
type Config struct {
	DataDir string
	DBPath  string
}

// Load returns the application configuration, creating the data directory if needed.
func Load() (*Config, error) {
	dataDir, err := dataDirectory()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, DBName),
	}, nil
}

// dataDirectory returns the path to ~/.gitm/
func dataDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "."+AppName), nil
}
