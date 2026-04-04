package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/auth"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

// errorWriter always returns an error when Write is called
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func TestPrintVersion(t *testing.T) {
	tests := []struct {
		name            string
		version         string
		commit          string
		date            string
		expectedContent []string
	}{
		{
			name:            "dev version",
			version:         "dev",
			commit:          "none",
			date:            "unknown",
			expectedContent: []string{"rivian-ls version dev"},
		},
		{
			name:    "release version with build info",
			version: "v1.0.0",
			commit:  "abc123def",
			date:    "2026-01-14T12:34:56Z",
			expectedContent: []string{
				"rivian-ls version v1.0.0",
				"commit: abc123def",
				"built:  2026-01-14T12:34:56Z",
			},
		},
		{
			name:    "version with commit only",
			version: "v0.1.0",
			commit:  "deadbeef",
			date:    "unknown",
			expectedContent: []string{
				"rivian-ls version v0.1.0",
				"commit: deadbeef",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origVersion, origCommit, origDate := version, commit, date
			defer func() {
				version, commit, date = origVersion, origCommit, origDate
			}()

			// Set test values
			version, commit, date = tt.version, tt.commit, tt.date

			var buf bytes.Buffer
			if err := printVersion(&buf); err != nil {
				t.Fatalf("printVersion failed: %v", err)
			}

			output := buf.String()
			for _, expected := range tt.expectedContent {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got: %s", expected, output)
				}
			}

			// Verify commit/date are not printed when they have default values
			if tt.commit == "none" && strings.Contains(output, "commit:") {
				t.Errorf("Should not print commit when it's 'none', got: %s", output)
			}
			if tt.date == "unknown" && strings.Contains(output, "built:") {
				t.Errorf("Should not print date when it's 'unknown', got: %s", output)
			}
		})
	}
}

func TestPrintVersionError(t *testing.T) {
	ew := &errorWriter{}
	err := printVersion(ew)
	if err == nil {
		t.Error("Expected error when writer fails, got nil")
	}
}

// Note: Testing run() is complex due to TUI integration and terminal control.
// The TUI uses Bubble Tea which requires a real terminal for full functionality.
// We test individual components (auth, TUI views, etc.) separately instead.

func TestPrintVersionCommitError(t *testing.T) {
	// Save original values
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	// Set values so commit branch is taken
	version, commit, date = "v1.0.0", "abc123", "unknown"

	ew := &errorWriter{}
	err := printVersion(ew)
	if err == nil {
		t.Error("Expected error when writer fails on version line, got nil")
	}
}

func TestPrintVersionDateError(t *testing.T) {
	// Save original values
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	// Set values so date branch is taken
	version, commit, date = "v1.0.0", "none", "2026-01-14"

	ew := &errorWriter{}
	err := printVersion(ew)
	if err == nil {
		t.Error("Expected error when writer fails, got nil")
	}
}

func TestHasFlag(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		flag   string
		expect bool
	}{
		{"long flag present", []string{"--offline", "--format", "json"}, "offline", true},
		{"long flag absent", []string{"--format", "json"}, "offline", false},
		{"flag with value", []string{"--offline=true", "--format", "json"}, "offline", true},
		{"short flag", []string{"-offline"}, "offline", true},
		{"empty args", []string{}, "offline", false},
		{"similar but different flag", []string{"--offline-mode"}, "offline", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFlag(tt.args, tt.flag)
			if got != tt.expect {
				t.Errorf("hasFlag(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.expect)
			}
		})
	}
}

func TestCompletePendingOTP_RestoresSavedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	credCache, err := auth.NewCredentialsCache()
	if err != nil {
		t.Fatalf("NewCredentialsCache failed: %v", err)
	}

	pending := &auth.PendingOTP{
		Email:        "test@example.com",
		OTPToken:     "otp-token-123",
		CSRFToken:    "saved-csrf-token",
		AppSessionID: "saved-app-session",
	}
	if err := credCache.SavePendingOTP(pending); err != nil {
		t.Fatalf("SavePendingOTP failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		if got := r.Header.Get("a-sess"); got != pending.AppSessionID {
			t.Errorf("expected a-sess %q, got %q", pending.AppSessionID, got)
		}
		if got := r.Header.Get("csrf-token"); got != pending.CSRFToken {
			t.Errorf("expected csrf-token %q, got %q", pending.CSRFToken, got)
		}

		variables, _ := req["variables"].(map[string]any)
		if got := variables["email"]; got != pending.Email {
			t.Errorf("expected email %q, got %v", pending.Email, got)
		}
		if got := variables["otpCode"]; got != "123456" {
			t.Errorf("expected otpCode %q, got %v", "123456", got)
		}
		if got := variables["otpToken"]; got != pending.OTPToken {
			t.Errorf("expected otpToken %q, got %v", pending.OTPToken, got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"loginWithOTP": map[string]any{
					"__typename":       "MobileLoginResponse",
					"accessToken":      "unused-access-token",
					"refreshToken":     "refresh-token",
					"userSessionToken": "user-session-token",
				},
			},
		})
	}))
	defer server.Close()

	client := rivian.NewHTTPClient(rivian.WithBaseURL(server.URL))

	if err := completePendingOTP(context.Background(), client, credCache, "123456"); err != nil {
		t.Fatalf("completePendingOTP failed: %v", err)
	}

	if client.GetCSRFToken() != pending.CSRFToken {
		t.Errorf("expected restored CSRF token %q, got %q", pending.CSRFToken, client.GetCSRFToken())
	}
	if client.GetAppSessionID() != pending.AppSessionID {
		t.Errorf("expected restored app session %q, got %q", pending.AppSessionID, client.GetAppSessionID())
	}

	if loadedPending, err := credCache.LoadPendingOTP(); err != nil {
		t.Fatalf("LoadPendingOTP failed: %v", err)
	} else if loadedPending != nil {
		t.Fatalf("expected pending OTP to be deleted after success, got %+v", loadedPending)
	}

	loadedCreds, err := credCache.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedCreds == nil {
		t.Fatal("expected saved credentials, got nil")
	}
	if loadedCreds.Email != pending.Email {
		t.Errorf("expected saved email %q, got %q", pending.Email, loadedCreds.Email)
	}
	if loadedCreds.AccessToken != "user-session-token" {
		t.Errorf("expected saved access token %q, got %q", "user-session-token", loadedCreds.AccessToken)
	}
}

func TestCompletePendingOTP_KeepsPendingSessionOnFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	credCache, err := auth.NewCredentialsCache()
	if err != nil {
		t.Fatalf("NewCredentialsCache failed: %v", err)
	}

	pending := &auth.PendingOTP{
		Email:        "test@example.com",
		OTPToken:     "otp-token-123",
		CSRFToken:    "saved-csrf-token",
		AppSessionID: "saved-app-session",
		SavedAt:      time.Now(),
	}
	if err := credCache.SavePendingOTP(pending); err != nil {
		t.Fatalf("SavePendingOTP failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{
				{"message": "User is unauthenticated"},
			},
		})
	}))
	defer server.Close()

	client := rivian.NewHTTPClient(rivian.WithBaseURL(server.URL))

	err = completePendingOTP(context.Background(), client, credCache, "123456")
	if err == nil {
		t.Fatal("expected completePendingOTP to fail")
	}
	if !strings.Contains(err.Error(), "OTP submission failed") {
		t.Fatalf("expected OTP submission failure, got %v", err)
	}

	loadedPending, err := credCache.LoadPendingOTP()
	if err != nil {
		t.Fatalf("LoadPendingOTP failed: %v", err)
	}
	if loadedPending == nil {
		t.Fatal("expected pending OTP session to remain after failure")
	}
	if loadedPending.CSRFToken != pending.CSRFToken || loadedPending.AppSessionID != pending.AppSessionID {
		t.Fatalf("pending OTP session changed after failure: %+v", loadedPending)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(credCache.Path()), "pending_otp.json")); err != nil {
		t.Fatalf("expected pending_otp.json to remain on disk, got %v", err)
	}
}
