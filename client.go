package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Logger defines a simple logging interface.
type Logger interface {
	Printf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

// newDefaultLogger returns a Logger backed by the standard library log package,
// writing to stderr with a "[1money-sdk]" prefix. It is installed by WithDebug
// when the caller does not supply their own Logger.
func newDefaultLogger() Logger {
	return &stdLogger{l: log.New(os.Stderr, "[1money-sdk] ", log.LstdFlags|log.Lmsgprefix)}
}

type stdLogger struct{ l *log.Logger }

func (s *stdLogger) Printf(format string, v ...interface{}) { s.l.Printf(format, v...) }
func (s *stdLogger) Infof(format string, v ...interface{})  { s.l.Printf("INFO "+format, v...) }
func (s *stdLogger) Warnf(format string, v ...interface{})  { s.l.Printf("WARN "+format, v...) }
func (s *stdLogger) Errorf(format string, v ...interface{}) { s.l.Printf("ERROR "+format, v...) }

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
	mainnetEndpoint = "https://api.1money.network"
	testnetEndpoint = "https://api.testnet.1money.network"
	devnetEndpoint  = "https://api.devnet.1money.network"
	localEndpoint   = "http://127.0.0.1:18555"
)

type Client struct {
	baseHost       string
	httpclient     *http.Client
	logger         Logger
	hooks          []Hook // New field
	submissionMode SubmissionMode
	debug          bool
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
	// Verbose debug logging needs somewhere to write; install a default stderr
	// logger when the caller enabled debug without supplying their own Logger,
	// so WithDebug() works on its own.
	if client.debug && client.logger == nil {
		client.logger = newDefaultLogger()
	}
	return client
}

func NewClient() *Client {
	return newClientInternal(mainnetEndpoint)
}

func NewTestClient() *Client {
	return newClientInternal(testnetEndpoint)
}

func NewClientWithCustomUrl(url string, opts ...ClientOption) *Client {
	return newClientInternal(url, opts...)
}

func NewClientWithOpts(opts ...ClientOption) *Client {
	return newClientInternal(mainnetEndpoint, opts...)
}

func NewTestClientWithOpts(opts ...ClientOption) *Client {
	return newClientInternal(testnetEndpoint, opts...)
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

// WithDebug enables verbose request/response logging: for each call the client
// logs the method and URL (already at info level), the request body (for POST),
// and the response status and body. Output goes through the client's Logger; if
// none is set, WithDebug installs a default logger that writes to stderr.
//
// Request bodies contain the signature and business fields (amounts, addresses,
// memo) but never a private key — the Signer holds the key and only the
// resulting signature is sent — so enabling debug does not log secrets.
func WithDebug() ClientOption {
	return func(c *Client) { c.debug = true }
}

// SubmissionMode selects the native transaction signing scheme and REST write
// surface used by the Client.Transactions()/Tokens()/Accounts() submit methods.
//
// The zero value is domain-separated v2 (the default and recommended mode), so
// a client built with any existing constructor submits v2 without extra
// configuration. A submission never probes one mode and falls back to the
// other: retrying a signed transaction under a different scheme can create two
// transactions for the same nonce.
type SubmissionMode int

const (
	// SubmissionModeDomainSeparatedV2 signs with the issue-1038 domain-separated
	// scheme and POSTs to /v2. This is the default (zero value).
	SubmissionModeDomainSeparatedV2 SubmissionMode = iota
	// SubmissionModeLegacyV1 signs with the legacy scheme and POSTs to /v1.
	// Explicit opt-in only, for compatibility during the migration window.
	SubmissionModeLegacyV1
)

// WithSubmissionMode sets the native submission mode on the client.
func WithSubmissionMode(m SubmissionMode) ClientOption {
	return func(c *Client) { c.submissionMode = m }
}

// WithLegacyV1 is a convenience option equivalent to
// WithSubmissionMode(SubmissionModeLegacyV1).
func WithLegacyV1() ClientOption {
	return func(c *Client) { c.submissionMode = SubmissionModeLegacyV1 }
}

// mode returns the client's configured submission mode.
func (c *Client) mode() SubmissionMode { return c.submissionMode }

// debugf logs a verbose request/response line when WithDebug is enabled.
func (c *Client) debugf(format string, args ...interface{}) {
	if c.debug && c.logger != nil {
		c.logger.Printf("[DEBUG] "+format, args...)
	}
}

// GetMethod executes a GET request to the specified path and decodes the JSON response into the result.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// It uses `any` because the actual type of the response varies depending on the API endpoint.
func (client *Client) GetMethod(ctx context.Context, path string, result interface{}) error {
	fullURL := client.baseHost + path
	if client.logger != nil {
		client.logger.Infof("GET %s", fullURL)
	}
	client.debugf("--> GET %s", fullURL)

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
		reqErr := fmt.Errorf("failed to create request: %w", err)
		// Call PostRequest hooks even if NewRequestWithContext fails (resp is nil),
		// passing the same error the caller receives.
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "GET", fullURL, 0, nil, reqErr)
			}
		}
		return reqErr
	}

	resp, err := client.httpclient.Do(req)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("API GET request to %s failed: %v", fullURL, err)
		}
		reqErr := fmt.Errorf("api get failed to request path: %s, err: %w", path, err)
		// Call PostRequest hooks if client.httpclient.Do fails, passing the same
		// error the caller receives.
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "GET", fullURL, 0, nil, reqErr)
			}
		}
		return reqErr
	}
	return client.handleAPIResponse(ctx, "GET", fullURL, resp, result)
}

// PostMethod executes a POST request to the specified path with the given body (marshalled to JSON)
// and decodes the JSON response into the result.
// The body parameter can be any Go value that can be marshalled to JSON.
// The result parameter must be a pointer to a Go value suitable for JSON unmarshalling.
// Both use `any` because the actual types vary depending on the API endpoint and request data.
func (client *Client) PostMethod(ctx context.Context, path string, body interface{}, result interface{}) error {
	fullURL := client.baseHost + path
	if client.logger != nil {
		client.logger.Infof("POST %s", fullURL)
	}

	data, err := json.Marshal(body)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("Failed to marshal request for POST %s: %v", fullURL, err)
		}
		marshalErr := fmt.Errorf("failed to marshal request: %w", err)
		// Call PostRequest hooks if json.Marshal fails, passing the same error
		// the caller receives.
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "POST", fullURL, 0, nil, marshalErr)
			}
		}
		return marshalErr
	}
	client.debugf("--> POST %s body=%s", fullURL, data)

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
		reqErr := fmt.Errorf("api post failed to request path: %s, err: %w", path, err)
		// Call PostRequest hooks even if NewRequestWithContext fails, passing the
		// same error the caller receives.
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "POST", fullURL, 0, nil, reqErr)
			}
		}
		return reqErr
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpclient.Do(req)
	if err != nil {
		if client.logger != nil {
			client.logger.Errorf("API POST request to %s failed: %v", fullURL, err)
		}
		reqErr := fmt.Errorf("failed to request path: %s, err: %w", path, err)
		// Call PostRequest hooks if client.httpclient.Do fails, passing the same
		// error the caller receives.
		if len(client.hooks) > 0 {
			for _, hook := range client.hooks {
				hook.PostRequest(ctx, "POST", fullURL, 0, nil, reqErr)
			}
		}
		return reqErr
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
func (client *Client) handleAPIResponse(ctx context.Context, method string, url string, resp *http.Response, result interface{}) error {
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
	client.debugf("<-- %s %s status=%d body=%s", method, url, resp.StatusCode, bodyBytes)

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
