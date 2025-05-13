package onemoney_test

import (
	"errors"
	"fmt"
	onemoney "github.com/1Money-Co/1money-go-sdk"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTokenAccount(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	address := TestOperatorAddress
	token := MintAccount
	result, err := api.GetTokenAccount(address, token)
	if err != nil {
		t.Fatalf("GetTokenAccount failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.Balance == "" {
		t.Error("Expected Balance to be present")
	}
	if result.TokenAccountAddress == "" {
		t.Error("Expected TokenAccountAddress to be present")
	}
	t.Logf("Successfully retrieved token account: %s", result.TokenAccountAddress)
	t.Logf("Balance: %s", result.Balance)
	t.Logf("Nonce: %d", result.Nonce)
}

func TestGetAccountNonce(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	address := "0xeFd86F9EA9b981edA887f984C7883481Ec665b61"
	result, err := api.GetAccountNonce(address)
	if err != nil {
		t.Fatalf("GetAccountNonce failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	t.Logf("Successfully retrieved account nonce: %d", result.Nonce)
}

func TestErrorHandling(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := fmt.Fprintf(w, `{"error_code":"invalid_request","message":"Invalid request parameters"}`)
		if err != nil {
			return
		}
	}))
	defer server.Close()
	// Test GetAccountNonce with error response
	_, err := api.GetAccountNonce("0x123")
	// Check if the error is of type APIError
	var apiErr *onemoney.APIError
	ok := errors.As(err, &apiErr)
	if !ok {
		t.Fatalf("Expected APIError, got %T: %v", err, err)
	}
	// Check the error details
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, apiErr.StatusCode)
	}
	if apiErr.ErrorCode != "invalid_request" {
		t.Errorf("Expected error code 'invalid_request', got '%s'", apiErr.ErrorCode)
	}
	if apiErr.Message != "Invalid request parameters" {
		t.Errorf("Expected message 'Invalid request parameters', got '%s'", apiErr.Message)
	}
	t.Logf("Successfully tested error handling: %v", err)
}
