package rivian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// BaseURL is the base URL for the Rivian API
	BaseURL = "https://rivian.com"

	// GraphQL endpoint
	GraphQLEndpoint = "/api/gql/gateway/graphql"

	// User-Agent to use for requests
	UserAgent = "rivian-ls/0.1.0"

	// Apollo client name (required by Rivian API)
	ApolloClientName = "com.rivian.android.consumer"
)

// HTTPClient implements the Client interface using HTTP/GraphQL.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string

	mu             sync.RWMutex
	credentials    *Credentials
	csrfToken      string // CSRF token for requests
	appSessionID   string // App session ID (a-sess header)
	otpToken       string // OTP token for MFA flow
	email          string // Email for OTP submission
}

// NewHTTPClient creates a new Rivian HTTP client.
func NewHTTPClient(opts ...Option) *HTTPClient {
	client := &HTTPClient{
		baseURL:   BaseURL,
		userAgent: UserAgent,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// Option is a functional option for configuring the HTTP client.
type Option func(*HTTPClient)

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *HTTPClient) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *HTTPClient) {
		c.httpClient = httpClient
	}
}

// WithCredentials initializes the client with existing credentials.
func WithCredentials(creds *Credentials) Option {
	return func(c *HTTPClient) {
		c.credentials = creds
	}
}

// IsAuthenticated returns true if the client has valid credentials.
func (c *HTTPClient) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.credentials == nil {
		return false
	}

	// Check if token is expired (with 5 minute buffer)
	return time.Now().Add(5 * time.Minute).Before(c.credentials.ExpiresAt)
}

// GetCredentials returns a copy of the current credentials.
func (c *HTTPClient) GetCredentials() *Credentials {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.credentials == nil {
		return nil
	}

	// Return a copy to avoid race conditions
	creds := *c.credentials
	return &creds
}

// graphqlRequest represents a GraphQL request.
type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// graphqlResponse represents a GraphQL response.
type graphqlResponse struct {
	Data   json.RawMessage   `json:"data"`
	Errors []graphqlError    `json:"errors,omitempty"`
}

// graphqlError represents a GraphQL error.
type graphqlError struct {
	Message string `json:"message"`
	Path    []string `json:"path,omitempty"`
}

// doGraphQL executes a GraphQL query.
func (c *HTTPClient) doGraphQL(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	reqBody := graphqlRequest{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+GraphQLEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("apollographql-client-name", ApolloClientName)

	// Add Rivian-specific headers
	c.mu.RLock()
	if c.appSessionID != "" {
		req.Header.Set("a-sess", c.appSessionID)
	}
	if c.csrfToken != "" {
		req.Header.Set("csrf-token", c.csrfToken)
	}
	if c.credentials != nil && c.credentials.AccessToken != "" {
		req.Header.Set("u-sess", c.credentials.AccessToken)
	}
	c.mu.RUnlock()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	if result != nil {
		if err := json.Unmarshal(gqlResp.Data, result); err != nil {
			return fmt.Errorf("unmarshal data: %w", err)
		}
	}

	return nil
}
