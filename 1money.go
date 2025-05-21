package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Logger defines a simple logging interface.
type Logger interface {
	Printf(format string, v ...interface{})
}

const (
	apiBaseHost     = "https://api.1money.network"
	apiBaseHostTest = "https://api.testnet.1money.network"
)

const (
	// TestOperatorPrivateKey is a mock private key for testing purposes.
	// IMPORTANT: This is a placeholder and not a real private key. Do not use for actual transactions.
	TestOperatorPrivateKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	// TestOperatorAddress is a mock Ethereum address for testing.
	TestOperatorAddress    = "0x1234567890123456789012345678901234567890"
	// TestTokenAddress is a mock token address for testing.
	TestTokenAddress       = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	// Test2ndAddress is another mock Ethereum address for testing.
	Test2ndAddress         = "0x0987654321098765432109876543210987654321"
)

type Client struct {
	baseHost   string
	httpclient *http.Client
	logger     Logger
}

func newClientInternal(baseHost string, options ...ClientOption) *Client {
	client := &Client{
		baseHost: baseHost,
		httpclient: &http.Client{
			Timeout: 4 * time.Second,
		},
		// logger is nil by default
	}
	for _, opt := range options {
		opt(client)
	}
	return client
}

func NewClient() *Client {
	return newClientInternal(apiBaseHost)
}

func NewTestClient() *Client {
	return newClientInternal(apiBaseHostTest)
}

func NewClientWithOpts(opts ...ClientOption) *Client {
	return newClientInternal(apiBaseHost, opts...)
}

func NewTestClientWithOpts(opts ...ClientOption) *Client {
	return newClientInternal(apiBaseHostTest, opts...)
}

// ClientOption defines a function that configures a Client
type ClientOption func(*Client)

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpclient.Timeout = timeout
	}
}

func WithHTTPClient(httpclient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpclient = httpclient
	}
}

// WithLogger sets the logger for the Client.
func WithLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// GetMethod executes a GET request to the specified path and decodes the JSON response into the result.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// It uses `any` because the actual type of the response varies depending on the API endpoint.
func (client *Client) GetMethod(ctx context.Context, path string, result any) error {
	fullURL := client.baseHost + path
	if client.logger != nil {
		client.logger.Printf("GET %s", fullURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		if client.logger != nil {
			client.logger.Printf("Failed to create request for GET %s: %v", fullURL, err)
		}
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.httpclient.Do(req)
	if err != nil {
		if client.logger != nil {
			client.logger.Printf("API GET request to %s failed: %v", fullURL, err)
		}
		return fmt.Errorf("api get failed to request path: %s, err: %w", path, err)
	}
	return client.handleAPIResponse(resp, result)
}

// PostMethod executes a POST request to the specified path with the given body (marshalled to JSON)
// and decodes the JSON response into the result.
// The body parameter can be any Go value that can be marshalled to JSON.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// Both use `any` because the actual types vary depending on the API endpoint and request data.
func (client *Client) PostMethod(ctx context.Context, path string, body any, result any) error {
	fullURL := client.baseHost + path
	if client.logger != nil {
		client.logger.Printf("POST %s", fullURL)
	}
	data, err := json.Marshal(body)
	if err != nil {
		if client.logger != nil {
			client.logger.Printf("Failed to marshal request for POST %s: %v", fullURL, err)
		}
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewBuffer(data))
	if err != nil {
		if client.logger != nil {
			client.logger.Printf("Failed to create request for POST %s: %v", fullURL, err)
		}
		return fmt.Errorf("api post failed to request path: %s, err: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.httpclient.Do(req)
	if err != nil {
		if client.logger != nil {
			client.logger.Printf("API POST request to %s failed: %v", fullURL, err)
		}
		return fmt.Errorf("failed to request path: %s, err: %w", path, err)
	}
	return client.handleAPIResponse(resp, result)
}

// ErrorResponse represents the error response from the API
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// APIError is a custom error type that includes the error response details
type APIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("API error: status=%d, code=%s, message=%s", e.StatusCode, e.ErrorCode, e.Message)
	}
	return fmt.Sprintf("API error: status=%d", e.StatusCode)
}

// handleAPIResponse is a helper function to handle API responses consistently.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// It uses `any` because the actual type of the response varies depending on the API endpoint.
func (client *Client) handleAPIResponse(resp *http.Response, result any) error {
	defer resp.Body.Close()
	// If status code is OK, decode the response into the result
	if resp.StatusCode == http.StatusOK {
		if result != nil {
			if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
				if client.logger != nil {
					client.logger.Printf("Failed to decode response from %s %s: %v", resp.Request.Method, resp.Request.URL.String(), err)
				}
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}
		return nil
	}
	// For non-200 responses, try to parse the error response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if client.logger != nil {
			client.logger.Printf("Failed to read error response body from %s %s: %v", resp.Request.Method, resp.Request.URL.String(), err)
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to read error response: %v", err),
		}
	}
	// Try to parse the error response
	var errorResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errorResp); err != nil {
		if client.logger != nil {
			client.logger.Printf("Failed to unmarshal error response from %s %s (status %d): %v. Body: %s", resp.Request.Method, resp.Request.URL.String(), resp.StatusCode, err, string(bodyBytes))
		}
		// If we can't parse the error response, return a generic error
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	if client.logger != nil {
		client.logger.Printf("API Error from %s %s: status=%d, code=%s, message=%s", resp.Request.Method, resp.Request.URL.String(), resp.StatusCode, errorResp.ErrorCode, errorResp.Message)
	}
	// Return a structured error with the error details
	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  errorResp.ErrorCode,
		Message:    errorResp.Message,
	}
}
