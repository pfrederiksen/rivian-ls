package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration options for rivian-ls
type Config struct {
	// Authentication
	Email    string `yaml:"email"`
	Password string `yaml:"password"` // Usually left empty, prompt is preferred

	// Storage
	DBPath      string `yaml:"db_path"`
	TokenCache  string `yaml:"token_cache"`
	DisableStore bool   `yaml:"disable_store"`

	// Vehicle selection
	Vehicle int `yaml:"vehicle"` // 0-based index

	// Polling
	PollInterval time.Duration `yaml:"poll_interval"`

	// Output
	Quiet   bool `yaml:"quiet"`
	Verbose bool `yaml:"verbose"`
}

// Load loads configuration from multiple sources in priority order:
// 1. Environment variables
// 2. Config file (~/.config/rivian-ls/config.yaml)
// 3. Defaults
//
// Note: CLI flags are applied separately by the caller and take highest precedence
func Load() (*Config, error) {
	cfg := &Config{
		// Defaults
		DBPath:       defaultDBPath(),
		TokenCache:   defaultTokenCachePath(),
		Vehicle:      0,
		PollInterval: 30 * time.Second,
		Quiet:        false,
		Verbose:      false,
		DisableStore: false,
	}

	// Load from config file if it exists
	if err := cfg.loadFromFile(); err != nil {
		// Non-fatal: config file is optional
		_ = err
	}

	// Override with environment variables
	cfg.loadFromEnv()

	return cfg, nil
}

// loadFromFile loads configuration from ~/.config/rivian-ls/config.yaml
func (c *Config) loadFromFile() error {
	configPath := getConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Config file is optional
		}
		return fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() {
	if email := os.Getenv("RIVIAN_EMAIL"); email != "" {
		c.Email = email
	}

	if password := os.Getenv("RIVIAN_PASSWORD"); password != "" {
		c.Password = password
	}

	if dbPath := os.Getenv("RIVIAN_DB_PATH"); dbPath != "" {
		c.DBPath = dbPath
	}

	if tokenCache := os.Getenv("RIVIAN_TOKEN_CACHE"); tokenCache != "" {
		c.TokenCache = tokenCache
	}

	if os.Getenv("RIVIAN_DISABLE_STORE") == "true" {
		c.DisableStore = true
	}

	if os.Getenv("RIVIAN_QUIET") == "true" {
		c.Quiet = true
	}

	if os.Getenv("RIVIAN_VERBOSE") == "true" {
		c.Verbose = true
	}

	if interval := os.Getenv("RIVIAN_POLL_INTERVAL"); interval != "" {
		if duration, err := time.ParseDuration(interval); err == nil {
			c.PollInterval = duration
		}
	}
}

// getConfigPath returns the path to the config file
func getConfigPath() string {
	// Try XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "rivian-ls", "config.yaml")
	}

	// Fall back to ~/.config
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "rivian-ls", "config.yaml")
}

// defaultDBPath returns the default database path
func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "rivian-ls.db"
	}

	return filepath.Join(home, ".local", "share", "rivian-ls", "state.db")
}

// defaultTokenCachePath returns the default token cache path
func defaultTokenCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "credentials.json"
	}

	return filepath.Join(home, ".local", "share", "rivian-ls", "credentials.json")
}
