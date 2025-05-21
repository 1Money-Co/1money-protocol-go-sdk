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
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

// Hook defines an interface for intercepting client operations.
type Hook interface {
	// PreRequest is called before an HTTP request is made.
	// The body parameter may be nil if there is no body.
	PreRequest(ctx context.Context, method, url string, body []byte)
	// PostRequest is called after an HTTP request has completed.
	// responseBody may be nil. err may be nil if the request was successful.
	PostRequest(ctx context.Context, method, url string, statusCode int, responseBody []byte, err error)
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
	hooks      []Hook // New field
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

// WithHooks adds hook implementations to the Client.
func WithHooks(hooks ...Hook) ClientOption {
	return func(c *Client) {
		c.hooks = append(c.hooks, hooks...)
	}
}

// GetMethod executes a GET request to the specified path and decodes the JSON response into the result.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// It uses `any` because the actual type of the response varies depending on the API endpoint.
func (client *Client) GetMethod(ctx context.Context, path string, result any) error {
	fullURL := client.baseHost + path
	if client.logger != nil {
		client.logger.Infof("GET %s", fullURL)
	}

	if len(client.hooks) > 0 {
		for _, hook := range client.hooks {
			hook.PreRequest(ctx, "GET", fullURL, nil)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("Failed to create request for GET %s: %v", fullURL, err)
		}
		// Call PostRequest hooks even if NewRequestWithContext fails (though resp is nil)
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				// Pass nil for responseBody as there's no response, and err for the error
				hook.PostRequest(ctx, "GET", fullURL, 0, nil, err)
			}
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpclient.Do(req)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("API GET request to %s failed: %v", fullURL, err)
		}
		// Call PostRequest hooks if client.httpclient.Do fails
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				// Pass nil for responseBody as there's no response, and err for the error
				hook.PostRequest(ctx, "GET", fullURL, 0, nil, err)
			}
		}
		return fmt.Errorf("api get failed to request path: %s, err: %w", path, err)
	}
	return client.handleAPIResponse(ctx, "GET", fullURL, resp, result)
}

// PostMethod executes a POST request to the specified path with the given body (marshalled to JSON)
// and decodes the JSON response into the result.
// The body parameter can be any Go value that can be marshalled to JSON.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// Both use `any` because the actual types vary depending on the API endpoint and request data.
func (client *Client) PostMethod(ctx context.Context, path string, body any, result any) error {
	fullURL := client.baseHost + path
	if client.logger != nil {
		client.logger.Infof("POST %s", fullURL)
	}

	data, err := json.Marshal(body)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("Failed to marshal request for POST %s: %v", fullURL, err)
		}
		// Call PostRequest hooks if json.Marshal fails
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				// Pass data (which might be nil or partially formed) and err
				hook.PostRequest(ctx, "POST", fullURL, 0, nil, err)
			}
		}
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if len(client.hooks) > 0 {
		for _, hook := range client.hooks {
			hook.PreRequest(ctx, "POST", fullURL, data)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewBuffer(data))
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("Failed to create request for POST %s: %v", fullURL, err)
		}
		// Call PostRequest hooks even if NewRequestWithContext fails
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "POST", fullURL, 0, nil, err)
			}
		}
		return fmt.Errorf("api post failed to request path: %s, err: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpclient.Do(req)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("API POST request to %s failed: %v", fullURL, err)
		}
		// Call PostRequest hooks if client.httpclient.Do fails
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "POST", fullURL, 0, nil, err)
			}
		}
		return fmt.Errorf("failed to request path: %s, err: %w", path, err)
	}
	return client.handleAPIResponse(ctx, "POST", fullURL, resp, result)
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
func (client *Client) handleAPIResponse(ctx context.Context, method string, url string, resp *http.Response, result any) error {
	defer resp.Body.Close()

	var processingErr error
	var bodyBytes []byte

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("Failed to read response body from %s %s: %v", method, url, err)
		}
		processingErr = &APIError{
			StatusCode: resp.StatusCode, // Could be 0 if error happened before getting status
			Message:    fmt.Sprintf("failed to read response body: %v", err),
		}
		// Call PostRequest hooks before returning
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, method, url, resp.StatusCode, nil, processingErr)
			}
		}
		return processingErr
	}

	// If status code is OK, decode the response into the result
	if resp.StatusCode == http.StatusOK {
		if result != nil {
			if err := json.Unmarshal(bodyBytes, result); err != nil {
				if client.logger != nil {
					client.logger.Errorf("Failed to decode response from %s %s: %v. Body: %s", method, url, err, string(bodyBytes))
				}
				processingErr = fmt.Errorf("failed to decode response: %w. Body: %s", err, string(bodyBytes))
			}
		}
		// processingErr remains nil if decode is successful
	} else {
		// For non-200 responses, try to parse the error response
		var errorResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errorResp); err != nil {
			if client.logger != nil {
				client.logger.Errorf("Failed to unmarshal error response from %s %s (status %d): %v. Body: %s", method, url, resp.StatusCode, err, string(bodyBytes))
			}
			processingErr = &APIError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes)),
			}
		} else {
			if client.logger != nil {
				client.logger.Errorf("API Error from %s %s: status=%d, code=%s, message=%s", method, url, resp.StatusCode, errorResp.ErrorCode, errorResp.Message)
			}
			processingErr = &APIError{
				StatusCode: resp.StatusCode,
				ErrorCode:  errorResp.ErrorCode,
				Message:    errorResp.Message,
			}
		}
	}

	// Call PostRequest hooks before returning
	if len(client.hooks) > 0 {
		for _, hook := range client.hooks {
			hook.PostRequest(ctx, method, url, resp.StatusCode, bodyBytes, processingErr)
		}
	}
	return processingErr
}
