package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Test default configuration
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Vehicle != 0 {
		t.Errorf("Expected default vehicle 0, got %d", cfg.Vehicle)
	}

	if cfg.PollInterval != 30*time.Second {
		t.Errorf("Expected default poll interval 30s, got %v", cfg.PollInterval)
	}

	if cfg.Quiet {
		t.Error("Expected quiet to be false by default")
	}

	if cfg.Verbose {
		t.Error("Expected verbose to be false by default")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("RIVIAN_EMAIL", "test@example.com")
	_ = os.Setenv("RIVIAN_PASSWORD", "testpassword")
	_ = os.Setenv("RIVIAN_POLL_INTERVAL", "1m")
	_ = os.Setenv("RIVIAN_QUIET", "true")
	defer func() {
		_ = os.Unsetenv("RIVIAN_EMAIL")
		_ = os.Unsetenv("RIVIAN_PASSWORD")
		_ = os.Unsetenv("RIVIAN_POLL_INTERVAL")
		_ = os.Unsetenv("RIVIAN_QUIET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Email != "test@example.com" {
		t.Errorf("Expected email from env, got %s", cfg.Email)
	}

	if cfg.Password != "testpassword" {
		t.Errorf("Expected password from env, got %s", cfg.Password)
	}

	if cfg.PollInterval != time.Minute {
		t.Errorf("Expected poll interval 1m, got %v", cfg.PollInterval)
	}

	if !cfg.Quiet {
		t.Error("Expected quiet to be true")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "rivian-ls")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `email: file@example.com
poll_interval: 45s
verbose: true
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Override config path temporarily
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() { _ = os.Unsetenv("XDG_CONFIG_HOME") }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Email != "file@example.com" {
		t.Errorf("Expected email from file, got %s", cfg.Email)
	}

	if cfg.PollInterval != 45*time.Second {
		t.Errorf("Expected poll interval 45s, got %v", cfg.PollInterval)
	}

	if !cfg.Verbose {
		t.Error("Expected verbose to be true")
	}
}
