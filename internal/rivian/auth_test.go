package rivian

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticate_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != GraphQLEndpoint {
			t.Errorf("Expected path %s, got %s", GraphQLEndpoint, r.URL.Path)
		}

		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		callCount++

		// First call should be CreateCSRFToken
		if callCount == 1 {
			if !strings.Contains(req.Query, "CreateCSRFToken") {
				t.Error("Expected CreateCSRFToken mutation in first call")
			}

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"createCsrfToken": map[string]interface{}{
						"__typename":       "CSRFTokenResponse",
						"csrfToken":        "test-csrf-token",
						"appSessionToken":  "test-app-session",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}

		// Second call should be Login
		if callCount == 2 {
			if !strings.Contains(req.Query, "mutation Login") {
				t.Error("Expected Login mutation in second call")
			}

			// Verify headers are set
			if r.Header.Get("a-sess") != "test-app-session" {
				t.Errorf("Expected a-sess header %s, got %s", "test-app-session", r.Header.Get("a-sess"))
			}
			if r.Header.Get("csrf-token") != "test-csrf-token" {
				t.Errorf("Expected csrf-token header %s, got %s", "test-csrf-token", r.Header.Get("csrf-token"))
			}

			// Return successful login response (MobileLoginResponse)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"login": map[string]interface{}{
						"__typename":        "MobileLoginResponse",
						"accessToken":       "test-access-token",
						"refreshToken":      "test-refresh-token",
						"userSessionToken":  "test-user-session",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}

		t.Errorf("Unexpected call count: %d", callCount)
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	err := client.Authenticate(context.Background(), "test@example.com", "password")

	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls, got %d", callCount)
	}

	if !client.IsAuthenticated() {
		t.Error("Client should be authenticated after successful login")
	}

	creds := client.GetCredentials()
	if creds == nil {
		t.Fatal("Credentials should not be nil")
	}
	if creds.AccessToken != "test-user-session" {
		t.Errorf("Expected access token %s, got %s", "test-user-session", creds.AccessToken)
	}
	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("Expected refresh token %s, got %s", "test-refresh-token", creds.RefreshToken)
	}
}

func TestAuthenticate_OTPRequired(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		callCount++

		// First call: CreateCSRFToken
		if callCount == 1 {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"createCsrfToken": map[string]interface{}{
						"__typename":       "CSRFTokenResponse",
						"csrfToken":        "test-csrf-token",
						"appSessionToken":  "test-app-session",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}

		// Second call: Login returns MFA required (MobileMFALoginResponse)
		if callCount == 2 {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"login": map[string]interface{}{
						"__typename": "MobileMFALoginResponse",
						"otpToken":   "otp-token-123",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))
	err := client.Authenticate(context.Background(), "test@example.com", "password")

	if err == nil {
		t.Fatal("Expected OTPRequiredError, got nil")
	}

	otpErr, ok := err.(*OTPRequiredError)
	if !ok {
		t.Fatalf("Expected OTPRequiredError, got %T", err)
	}

	if otpErr.SessionID != "otp-token-123" {
		t.Errorf("Expected session ID %s, got %s", "otp-token-123", otpErr.SessionID)
	}

	if client.IsAuthenticated() {
		t.Error("Client should not be authenticated when OTP is required")
	}

	// Verify email is stored for OTP submission
	client.mu.RLock()
	if client.email != "test@example.com" {
		t.Errorf("Expected email to be stored, got %s", client.email)
	}
	client.mu.RUnlock()
}

func TestSubmitOTP_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		callCount++

		// Calls 1-2: CreateCSRFToken and Login (OTP required)
		if callCount == 1 {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"createCsrfToken": map[string]interface{}{
						"__typename":       "CSRFTokenResponse",
						"csrfToken":        "test-csrf-token",
						"appSessionToken":  "test-app-session",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}

		if callCount == 2 {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"login": map[string]interface{}{
						"__typename": "MobileMFALoginResponse",
						"otpToken":   "otp-token-123",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}

		// Call 3: LoginWithOTP
		if callCount == 3 {
			if !strings.Contains(req.Query, "LoginWithOTP") {
				t.Error("Expected LoginWithOTP mutation")
			}

			// Verify the email is passed in variables
			if req.Variables["email"] != "test@example.com" {
				t.Errorf("Expected email %s, got %v", "test@example.com", req.Variables["email"])
			}
			if req.Variables["otpCode"] != "123456" {
				t.Errorf("Expected OTP code %s, got %v", "123456", req.Variables["otpCode"])
			}
			if req.Variables["otpToken"] != "otp-token-123" {
				t.Errorf("Expected OTP token %s, got %v", "otp-token-123", req.Variables["otpToken"])
			}

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"loginWithOTP": map[string]interface{}{
						"__typename":        "MobileLoginResponse",
						"accessToken":       "test-access-token",
						"refreshToken":      "test-refresh-token",
						"userSessionToken":  "test-user-session",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}
	}))
	defer server.Close()

	client := NewHTTPClient(WithBaseURL(server.URL))

	// First login triggers OTP
	err := client.Authenticate(context.Background(), "test@example.com", "password")
	if err == nil {
		t.Fatal("Expected OTPRequiredError")
	}

	// Submit OTP
	err = client.SubmitOTP(context.Background(), "123456")
	if err != nil {
		t.Fatalf("SubmitOTP failed: %v", err)
	}

	if !client.IsAuthenticated() {
		t.Error("Client should be authenticated after successful OTP submission")
	}

	// Verify email and otpToken are cleared after successful OTP
	client.mu.RLock()
	if client.email != "" {
		t.Error("Email should be cleared after successful OTP")
	}
	if client.otpToken != "" {
		t.Error("OTP token should be cleared after successful OTP")
	}
	client.mu.RUnlock()
}

func TestSubmitOTP_NoSession(t *testing.T) {
	client := NewHTTPClient()
	err := client.SubmitOTP(context.Background(), "123456")

	if err == nil {
		t.Fatal("Expected error when submitting OTP without active session")
	}

	if !strings.Contains(err.Error(), "no OTP session active") {
		t.Errorf("Expected 'no OTP session active' error, got: %v", err)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if strings.Contains(req.Query, "RefreshAccessToken") {
			// Verify refresh token is passed
			if req.Variables["refreshToken"] != "old-refresh-token" {
				t.Errorf("Expected refresh token %s, got %v", "old-refresh-token", req.Variables["refreshToken"])
			}

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"refreshAccessToken": map[string]interface{}{
						"accessToken":  "new-access-token",
						"refreshToken": "new-refresh-token",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		}
	}))
	defer server.Close()

	// Create client with existing credentials
	client := NewHTTPClient(
		WithBaseURL(server.URL),
		WithCredentials(&Credentials{
			AccessToken:  "old-access-token",
			RefreshToken: "old-refresh-token",
			ExpiresAt:    time.Now().Add(-1 * time.Hour), // Expired
			UserID:       "test-user-id",
		}),
	)

	err := client.RefreshToken(context.Background())
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	creds := client.GetCredentials()
	if creds.AccessToken != "new-access-token" {
		t.Errorf("Expected new access token, got %s", creds.AccessToken)
	}
	if creds.RefreshToken != "new-refresh-token" {
		t.Errorf("Expected new refresh token, got %s", creds.RefreshToken)
	}
	// Verify UserID is preserved
	if creds.UserID != "test-user-id" {
		t.Errorf("Expected user ID to be preserved, got %s", creds.UserID)
	}
}

func TestIsAuthenticated(t *testing.T) {
	tests := []struct {
		name        string
		credentials *Credentials
		want        bool
	}{
		{
			name:        "no credentials",
			credentials: nil,
			want:        false,
		},
		{
			name: "expired token",
			credentials: &Credentials{
				AccessToken:  "token",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(-1 * time.Hour),
			},
			want: false,
		},
		{
			name: "valid token",
			credentials: &Credentials{
				AccessToken:  "token",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(1 * time.Hour),
			},
			want: true,
		},
		{
			name: "token expiring soon (within 5 minutes)",
			credentials: &Credentials{
				AccessToken:  "token",
				RefreshToken: "refresh",
				ExpiresAt:    time.Now().Add(3 * time.Minute),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(WithCredentials(tt.credentials))
			got := client.IsAuthenticated()
			if got != tt.want {
				t.Errorf("IsAuthenticated() = %v, want %v", got, tt.want)
			}
		})
	}
}
