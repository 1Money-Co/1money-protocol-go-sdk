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
	TestOperatorPrivateKey = "0xed90b5cb37fd3f17148f55488c607a932ca8c672cce38a0810b72522f0672408"
	TestOperatorAddress    = "0x276dfcc7e502f4a3330857beee0fa574c499242b"
	TestMintAccount        = "0x4e34917ebEc4df28CC5ad641637e321Aa590E53f"
	Test2ndPrivateKey      = "0xb1c49ed15a19a21541cd71a0837c75194756cbe81ac13c14e31213d766e84e7a"
	Test2ndPublicKey       = "0x22b8a59ae8f55f3955f356118374fcdde5a6e0143a30a2e80a154877bc3e9a1b"
	Test2ndAddress         = "0x1DFa71eC8284F0F835EDbfaEA458d38bCff446d6"
)

type Client struct {
	baseUrl string
	client  *http.Client
}

func New() *Client {
	return &Client{
		baseUrl: apiBaseUrl,
		client:  http.DefaultClient,
	}
}

func NewTest() *Client {
	return &Client{
		baseUrl: apiBaseUrlTest,
		client:  http.DefaultClient,
	}
}

func (api *Client) GetMethod(path string, result any) error {
	req, err := http.NewRequest("GET", api.baseUrl+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return fmt.Errorf("api get failed to request path: %s, err: %w", path, err)
	}
	return handleAPIResponse(resp, &result)
}

func (api *Client) PostMethod(path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequest("POST", api.baseUrl+path, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("api post failed to request path: %s, err: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.client.Do(req)
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
