package rivian

import (
	"context"
	"fmt"
	"time"
)

const (
	createCSRFTokenMutation = `
		mutation CreateCSRFToken {
			createCsrfToken {
				__typename
				csrfToken
				appSessionToken
			}
		}
	`

	loginMutation = `
		mutation Login($email: String!, $password: String!) {
			login(email: $email, password: $password) {
				__typename
				... on MobileLoginResponse {
					accessToken
					refreshToken
					userSessionToken
				}
				... on MobileMFALoginResponse {
					otpToken
				}
			}
		}
	`

	loginWithOTPMutation = `
		mutation LoginWithOTP($email: String!, $otpCode: String!, $otpToken: String!) {
			loginWithOTP(email: $email, otpCode: $otpCode, otpToken: $otpToken) {
				__typename
				accessToken
				refreshToken
				userSessionToken
			}
		}
	`

	refreshTokenMutation = `
		mutation RefreshAccessToken($refreshToken: String!) {
			refreshAccessToken(refreshToken: $refreshToken) {
				accessToken
				refreshToken
			}
		}
	`
)

// csrfTokenResponse represents the response from CreateCSRFToken mutation.
type csrfTokenResponse struct {
	CreateCsrfToken struct {
		Typename        string `json:"__typename"`
		CsrfToken       string `json:"csrfToken"`
		AppSessionToken string `json:"appSessionToken"`
	} `json:"createCsrfToken"`
}

// loginResponse represents the response from the Login mutation.
type loginResponse struct {
	Login struct {
		Typename         string  `json:"__typename"`
		AccessToken      *string `json:"accessToken,omitempty"`
		RefreshToken     *string `json:"refreshToken,omitempty"`
		UserSessionToken *string `json:"userSessionToken,omitempty"`
		OtpToken         *string `json:"otpToken,omitempty"`
	} `json:"login"`
}

// loginWithOTPResponse represents the response from LoginWithOTP mutation.
type loginWithOTPResponse struct {
	LoginWithOTP struct {
		Typename         string `json:"__typename"`
		AccessToken      string `json:"accessToken"`
		RefreshToken     string `json:"refreshToken"`
		UserSessionToken string `json:"userSessionToken"`
	} `json:"loginWithOTP"`
}

// refreshTokenResponse represents the response from token refresh.
type refreshTokenResponse struct {
	RefreshAccessToken struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	} `json:"refreshAccessToken"`
}

// Authenticate performs login with email and password.
// Returns OTPRequiredError if MFA is enabled.
func (c *HTTPClient) Authenticate(ctx context.Context, email, password string) error {
	// Step 1: Get CSRF token and app session
	var csrfResp csrfTokenResponse
	if err := c.doGraphQL(ctx, createCSRFTokenMutation, nil, &csrfResp); err != nil {
		return fmt.Errorf("create CSRF token: %w", err)
	}

	c.mu.Lock()
	c.csrfToken = csrfResp.CreateCsrfToken.CsrfToken
	c.appSessionID = csrfResp.CreateCsrfToken.AppSessionToken
	c.email = email // Store email for OTP flow
	c.mu.Unlock()

	// Step 2: Attempt login
	variables := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	var loginResp loginResponse
	if err := c.doGraphQL(ctx, loginMutation, variables, &loginResp); err != nil {
		return fmt.Errorf("login request: %w", err)
	}

	// Check response type
	if loginResp.Login.Typename == "MobileMFALoginResponse" {
		// MFA required
		if loginResp.Login.OtpToken != nil {
			c.mu.Lock()
			c.otpToken = *loginResp.Login.OtpToken
			c.mu.Unlock()
			return &OTPRequiredError{SessionID: *loginResp.Login.OtpToken}
		}
		return fmt.Errorf("MFA required but no OTP token received")
	}

	// MobileLoginResponse - store credentials
	if loginResp.Login.AccessToken == nil || loginResp.Login.RefreshToken == nil {
		return fmt.Errorf("login response missing tokens")
	}

	c.mu.Lock()
	c.credentials = &Credentials{
		AccessToken:  *loginResp.Login.UserSessionToken, // u-sess uses userSessionToken
		RefreshToken: *loginResp.Login.RefreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour), // Rivian tokens typically last 24 hours
	}
	c.mu.Unlock()

	return nil
}

// SubmitOTP submits a one-time password for MFA authentication.
func (c *HTTPClient) SubmitOTP(ctx context.Context, code string) error {
	c.mu.RLock()
	otpToken := c.otpToken
	email := c.email
	c.mu.RUnlock()

	if otpToken == "" {
		return fmt.Errorf("no OTP session active - call Authenticate first")
	}

	if email == "" {
		return fmt.Errorf("no email stored - call Authenticate first")
	}

	variables := map[string]interface{}{
		"email":    email,
		"otpCode":  code,
		"otpToken": otpToken,
	}

	var resp loginWithOTPResponse
	if err := c.doGraphQL(ctx, loginWithOTPMutation, variables, &resp); err != nil {
		return fmt.Errorf("submit OTP: %w", err)
	}

	// Store credentials
	c.mu.Lock()
	c.credentials = &Credentials{
		AccessToken:  resp.LoginWithOTP.UserSessionToken, // u-sess uses userSessionToken
		RefreshToken: resp.LoginWithOTP.RefreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	c.otpToken = "" // Clear OTP token
	c.email = ""    // Clear stored email
	c.mu.Unlock()

	return nil
}

// RefreshToken refreshes the access token using the stored refresh token.
func (c *HTTPClient) RefreshToken(ctx context.Context) error {
	c.mu.RLock()
	if c.credentials == nil || c.credentials.RefreshToken == "" {
		c.mu.RUnlock()
		return fmt.Errorf("no refresh token available")
	}
	refreshToken := c.credentials.RefreshToken
	c.mu.RUnlock()

	variables := map[string]interface{}{
		"refreshToken": refreshToken,
	}

	var resp refreshTokenResponse
	if err := c.doGraphQL(ctx, refreshTokenMutation, variables, &resp); err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}

	// Update credentials
	c.mu.Lock()
	c.credentials = &Credentials{
		AccessToken:  resp.RefreshAccessToken.AccessToken,
		RefreshToken: resp.RefreshAccessToken.RefreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		UserID:       c.credentials.UserID, // Preserve user ID
	}
	c.mu.Unlock()

	return nil
}
