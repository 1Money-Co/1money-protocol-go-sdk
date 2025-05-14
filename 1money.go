package onemoney

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	apiBaseUrl     = "https://api.1money.network"
	apiBaseUrlTest = "https://api.testnet.1money.network"
)

const (
	TestOperatorPrivateKey = ""
	TestOperatorAddress    = ""
	TestTokenAddress       = ""
	Test2ndAddress         = ""
)

type Client struct {
	baseUrl    string
	httpclient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseUrl:    apiBaseUrl,
		httpclient: http.DefaultClient,
	}
}

func NewTestClient() *Client {
	return &Client{
		baseUrl:    apiBaseUrlTest,
		httpclient: http.DefaultClient,
	}
}

func (client *Client) GetMethod(path string, result any) error {
	req, err := http.NewRequest("GET", client.baseUrl+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.httpclient.Do(req)
	if err != nil {
		return fmt.Errorf("api get failed to request path: %s, err: %w", path, err)
	}
	return handleAPIResponse(resp, &result)
}

func (client *Client) PostMethod(path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequest("POST", client.baseUrl+path, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("api post failed to request path: %s, err: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.httpclient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request path: %s, err: %w", path, err)
	}
	return handleAPIResponse(resp, &result)
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

// handleAPIResponse is a helper function to handle API responses consistently
func handleAPIResponse(resp *http.Response, result any) error {
	defer resp.Body.Close()
	// If status code is OK, decode the response into the result
	if resp.StatusCode == http.StatusOK {
		if result != nil {
			if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}
		return nil
	}
	// For non-200 responses, try to parse the error response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to read error response: %v", err),
		}
	}
	// Try to parse the error response
	var errorResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errorResp); err != nil {
		// If we can't parse the error response, return a generic error
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
		}
	}
	// Return a structured error with the error details
	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  errorResp.ErrorCode,
		Message:    errorResp.Message,
	}
}
