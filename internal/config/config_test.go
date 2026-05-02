package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandreafj/gitm/internal/config"
)

func TestLoad(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.DataDir == "" {
		t.Error("DataDir is empty")
	}

	if cfg.DBPath == "" {
		t.Error("DBPath is empty")
	}
}

func TestLoadCreatesDirIfMissing(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	info, err := os.Stat(cfg.DataDir)
	if err != nil {
		t.Fatalf("DataDir does not exist: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("DataDir is not a directory")
	}
}

func TestLoadDBPathFormat(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !filepath.IsAbs(cfg.DBPath) {
		t.Errorf("DBPath is not absolute: %q", cfg.DBPath)
	}

	basename := filepath.Base(cfg.DBPath)
	if basename != config.DBName {
		t.Errorf("DBPath basename = %q, want %q", basename, config.DBName)
	}
}

func TestLoadDBPathInDataDir(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	expectedPath := filepath.Join(cfg.DataDir, config.DBName)
	if cfg.DBPath != expectedPath {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, expectedPath)
	}
}

func TestLoadConfigValuesNotEmpty(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DataDir == "" {
		t.Error("DataDir should not be empty")
	}

	if cfg.DBPath == "" {
		t.Error("DBPath should not be empty")
	}

	// DataDir should contain the app name
	if !contains(cfg.DataDir, "."+config.AppName) {
		t.Errorf("DataDir should contain '.%s': %q", config.AppName, cfg.DataDir)
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
