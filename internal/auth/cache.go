package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

// CachedCredentials represents credentials stored on disk
type CachedCredentials struct {
	Email        string    `json:"email"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	SavedAt      time.Time `json:"saved_at"`
}

// CredentialsCache manages persistent credential storage
type CredentialsCache struct {
	path string
}

// NewCredentialsCache creates a new credentials cache
func NewCredentialsCache() (*CredentialsCache, error) {
	// Use ~/.config/rivian-ls/credentials.json on Unix
	// or %APPDATA%/rivian-ls/credentials.json on Windows
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "rivian-ls")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}

	credPath := filepath.Join(configDir, "credentials.json")

	return &CredentialsCache{path: credPath}, nil
}

// Load reads cached credentials from disk
func (c *CredentialsCache) Load() (*CachedCredentials, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cached credentials
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds CachedCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	return &creds, nil
}

// Save writes credentials to disk
func (c *CredentialsCache) Save(email string, creds *rivian.Credentials) error {
	cached := CachedCredentials{
		Email:        email,
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		ExpiresAt:    creds.ExpiresAt,
		SavedAt:      time.Now(),
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// Write with restricted permissions (0600 = owner read/write only)
	if err := os.WriteFile(c.path, data, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

// Delete removes cached credentials
func (c *CredentialsCache) Delete() error {
	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return nil
}

// IsValid checks if cached credentials are still valid
func (c *CachedCredentials) IsValid() bool {
	// Consider expired if less than 5 minutes remaining
	return time.Until(c.ExpiresAt) > 5*time.Minute
}

// ToRivianCredentials converts to rivian.Credentials
func (c *CachedCredentials) ToRivianCredentials() *rivian.Credentials {
	return &rivian.Credentials{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
		ExpiresAt:    c.ExpiresAt,
	}
}

// Path returns the path to the credentials file
func (c *CredentialsCache) Path() string {
	return c.path
}
