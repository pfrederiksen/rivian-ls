package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

func TestNewCredentialsCache(t *testing.T) {
	cache, err := NewCredentialsCache()
	if err != nil {
		t.Fatalf("NewCredentialsCache failed: %v", err)
	}

	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}

	if cache.Path() == "" {
		t.Error("Cache path is empty")
	}

	// Path should be in home directory
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".config", "rivian-ls", "credentials.json")
	if cache.Path() != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, cache.Path())
	}
}

func TestCredentialsCache_SaveAndLoad(t *testing.T) {
	// Create temporary cache
	tmpDir := t.TempDir()
	cache := &CredentialsCache{
		path: filepath.Join(tmpDir, "creds.json"),
	}

	// Create test credentials
	expiresAt := time.Now().Add(24 * time.Hour)
	creds := &rivian.Credentials{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    expiresAt,
	}

	// Save credentials
	err := cache.Save("test@example.com", creds)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created with correct permissions
	info, err := os.Stat(cache.Path())
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	// Check file permissions (should be 0600)
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Load credentials
	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected credentials, got nil")
	}

	// Verify loaded credentials
	if loaded.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", loaded.Email)
	}
	if loaded.AccessToken != "test-access-token" {
		t.Errorf("Expected access token test-access-token, got %s", loaded.AccessToken)
	}
	if loaded.RefreshToken != "test-refresh-token" {
		t.Errorf("Expected refresh token test-refresh-token, got %s", loaded.RefreshToken)
	}

	// ExpiresAt should be within 1 second of original
	if loaded.ExpiresAt.Sub(expiresAt).Abs() > time.Second {
		t.Errorf("ExpiresAt mismatch: expected %v, got %v", expiresAt, loaded.ExpiresAt)
	}
}

func TestCredentialsCache_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &CredentialsCache{
		path: filepath.Join(tmpDir, "nonexistent.json"),
	}

	loaded, err := cache.Load()
	if err != nil {
		t.Errorf("Load should not error on missing file, got: %v", err)
	}

	if loaded != nil {
		t.Error("Expected nil for non-existent file")
	}
}

func TestCredentialsCache_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &CredentialsCache{
		path: filepath.Join(tmpDir, "invalid.json"),
	}

	// Write invalid JSON
	err := os.WriteFile(cache.Path(), []byte("not valid json"), 0600)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	loaded, err := cache.Load()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	if loaded != nil {
		t.Error("Expected nil for invalid JSON")
	}
}

func TestCredentialsCache_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &CredentialsCache{
		path: filepath.Join(tmpDir, "creds.json"),
	}

	// Save credentials first
	creds := &rivian.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	err := cache.Save("test@example.com", creds)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete credentials
	err = cache.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is gone
	_, err = os.Stat(cache.Path())
	if !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}

	// Delete again should not error
	err = cache.Delete()
	if err != nil {
		t.Errorf("Delete of non-existent file should not error, got: %v", err)
	}
}

func TestCachedCredentials_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "valid - expires in 1 hour",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  true,
		},
		{
			name:      "valid - expires in 10 minutes",
			expiresAt: time.Now().Add(10 * time.Minute),
			expected:  true,
		},
		{
			name:      "invalid - expires in 4 minutes",
			expiresAt: time.Now().Add(4 * time.Minute),
			expected:  false,
		},
		{
			name:      "invalid - expired 1 hour ago",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  false,
		},
		{
			name:      "invalid - expired just now",
			expiresAt: time.Now(),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &CachedCredentials{
				Email:        "test@example.com",
				AccessToken:  "test-token",
				RefreshToken: "test-refresh",
				ExpiresAt:    tt.expiresAt,
				SavedAt:      time.Now(),
			}

			if got := creds.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestCachedCredentials_ToRivianCredentials(t *testing.T) {
	expiresAt := time.Now().Add(1 * time.Hour)
	cached := &CachedCredentials{
		Email:        "test@example.com",
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    expiresAt,
		SavedAt:      time.Now(),
	}

	rivianCreds := cached.ToRivianCredentials()

	if rivianCreds == nil {
		t.Fatal("Expected credentials, got nil")
	}

	if rivianCreds.AccessToken != cached.AccessToken {
		t.Errorf("AccessToken mismatch: expected %s, got %s",
			cached.AccessToken, rivianCreds.AccessToken)
	}

	if rivianCreds.RefreshToken != cached.RefreshToken {
		t.Errorf("RefreshToken mismatch: expected %s, got %s",
			cached.RefreshToken, rivianCreds.RefreshToken)
	}

	if !rivianCreds.ExpiresAt.Equal(cached.ExpiresAt) {
		t.Errorf("ExpiresAt mismatch: expected %v, got %v",
			cached.ExpiresAt, rivianCreds.ExpiresAt)
	}
}

func TestCredentialsCache_Path(t *testing.T) {
	testPath := "/tmp/test/creds.json"
	cache := &CredentialsCache{path: testPath}

	if cache.Path() != testPath {
		t.Errorf("Expected path %s, got %s", testPath, cache.Path())
	}
}

func TestPendingOTP_SaveLoadDelete(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &CredentialsCache{path: filepath.Join(tmpDir, "credentials.json")}

	// Save pending OTP
	pending := &PendingOTP{
		Email:        "test@example.com",
		OTPToken:     "otp-token-123",
		CSRFToken:    "csrf-token-456",
		AppSessionID: "app-session-789",
	}
	if err := cache.SavePendingOTP(pending); err != nil {
		t.Fatalf("SavePendingOTP failed: %v", err)
	}

	// Load it back
	loaded, err := cache.LoadPendingOTP()
	if err != nil {
		t.Fatalf("LoadPendingOTP failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Expected pending OTP, got nil")
	}
	if loaded.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", loaded.Email, "test@example.com")
	}
	if loaded.OTPToken != "otp-token-123" {
		t.Errorf("OTPToken = %q, want %q", loaded.OTPToken, "otp-token-123")
	}
	if loaded.CSRFToken != "csrf-token-456" {
		t.Errorf("CSRFToken = %q, want %q", loaded.CSRFToken, "csrf-token-456")
	}
	if loaded.AppSessionID != "app-session-789" {
		t.Errorf("AppSessionID = %q, want %q", loaded.AppSessionID, "app-session-789")
	}
	if !loaded.IsValid() {
		t.Error("Freshly saved pending OTP should be valid")
	}

	// Delete it
	if err := cache.DeletePendingOTP(); err != nil {
		t.Fatalf("DeletePendingOTP failed: %v", err)
	}

	// Load should return nil now
	loaded, err = cache.LoadPendingOTP()
	if err != nil {
		t.Fatalf("LoadPendingOTP after delete failed: %v", err)
	}
	if loaded != nil {
		t.Error("Expected nil after delete")
	}
}

func TestPendingOTP_IsValid(t *testing.T) {
	fresh := &PendingOTP{SavedAt: time.Now()}
	if !fresh.IsValid() {
		t.Error("Fresh pending OTP should be valid")
	}

	expired := &PendingOTP{SavedAt: time.Now().Add(-15 * time.Minute)}
	if expired.IsValid() {
		t.Error("15-minute-old pending OTP should be expired")
	}
}

func TestPendingOTP_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &CredentialsCache{path: filepath.Join(tmpDir, "credentials.json")}

	loaded, err := cache.LoadPendingOTP()
	if err != nil {
		t.Fatalf("LoadPendingOTP should not error for missing file: %v", err)
	}
	if loaded != nil {
		t.Error("Expected nil for non-existent file")
	}
}
