package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-config-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Path != "wikigraph.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "wikigraph.db")
	}
	if cfg.Scraper.RateLimit != 100.0 {
		t.Errorf("Scraper.RateLimit = %v, want %v", cfg.Scraper.RateLimit, 100.0)
	}
	if cfg.Scraper.MaxDepth != 3 {
		t.Errorf("Scraper.MaxDepth = %d, want %d", cfg.Scraper.MaxDepth, 3)
	}
	if cfg.Scraper.RequestTimeout != 30*time.Second {
		t.Errorf("Scraper.RequestTimeout = %v, want %v", cfg.Scraper.RequestTimeout, 30*time.Second)
	}
	if cfg.Scraper.MaxConcurrent != 30 {
		t.Errorf("Scraper.MaxConcurrent = %d, want %d", cfg.Scraper.MaxConcurrent, 30)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-config-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `
database:
  path: /custom/path/data.db
scraper:
  rate_limit: 5.0
  max_depth: 5
log:
  level: debug
`
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Path != "/custom/path/data.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/custom/path/data.db")
	}
	if cfg.Scraper.RateLimit != 5.0 {
		t.Errorf("Scraper.RateLimit = %v, want %v", cfg.Scraper.RateLimit, 5.0)
	}
	if cfg.Scraper.MaxDepth != 5 {
		t.Errorf("Scraper.MaxDepth = %d, want %d", cfg.Scraper.MaxDepth, 5)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wikigraph-config-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.Setenv("WIKIGRAPH_DATABASE_PATH", "/env/override.db")
	os.Setenv("WIKIGRAPH_LOG_LEVEL", "warn")
	defer os.Unsetenv("WIKIGRAPH_DATABASE_PATH")
	defer os.Unsetenv("WIKIGRAPH_LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Path != "/env/override.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/env/override.db")
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "warn")
	}
}
