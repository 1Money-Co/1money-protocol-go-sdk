package onemoney

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockLogger is a simple mock for the Logger interface.
type mockLogger struct {
	mu     sync.Mutex
	printfCalls []string
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.printfCalls = append(m.printfCalls, fmt.Sprintf(format, v...))
}

func (m *mockLogger) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy
	calls := make([]string, len(m.printfCalls))
	copy(calls, m.printfCalls)
	return calls
}

func TestClient_WithLogger_GetMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/test_endpoint" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"status":"ok"}`)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"error_code":"NOT_FOUND", "message":"Endpoint not found"}`)
		}
	}))
	defer server.Close()

	logger := &mockLogger{}
	client := newClientInternal(server.URL, WithLogger(logger), WithTimeout(2*time.Second))

	var result struct {
		Status string `json:"status"`
	}

	err := client.GetMethod(context.Background(), "/v1/test_endpoint", &result)
	if err != nil {
		t.Fatalf("GetMethod failed: %v", err)
	}

	if result.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result.Status)
	}

	calls := logger.getCalls()
	if len(calls) == 0 {
		t.Fatal("Expected logger to be called, but it wasn't")
	}

	expectedLog := fmt.Sprintf("GET %s/v1/test_endpoint", server.URL)
	foundLog := false
	for _, call := range calls {
		if strings.Contains(call, expectedLog) {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Errorf("Expected log message containing '%s', got calls: %v", expectedLog, calls)
	}

	// Test error logging
	logger.printfCalls = []string{} // Reset calls
	err = client.GetMethod(context.Background(), "/v1/nonexistent_endpoint", &result)
	if err == nil {
		t.Fatal("Expected GetMethod to fail for nonexistent endpoint, but it didn't")
	}

	calls = logger.getCalls()
	if len(calls) < 2 { // Should have at least GET and API Error log
		t.Fatalf("Expected logger to be called multiple times for error case, got: %v", calls)
	}
	
	expectedErrorLog := fmt.Sprintf("API Error from GET %s/v1/nonexistent_endpoint: status=404", server.URL)
	foundErrorLog := false
	for _, call := range calls {
		if strings.Contains(call, expectedErrorLog) {
			foundErrorLog = true
			break
		}
	}
	if !foundErrorLog {
		t.Errorf("Expected error log message containing '%s', got calls: %v", expectedErrorLog, calls)
	}
}

func TestClient_WithLogger_PostMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/test_post_endpoint" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"status":"posted"}`)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"error_code":"NOT_FOUND", "message":"Endpoint not found"}`)
		}
	}))
	defer server.Close()

	logger := &mockLogger{}
	client := newClientInternal(server.URL, WithLogger(logger), WithTimeout(2*time.Second))

	requestBody := struct {
		Data string `json:"data"`
	}{Data: "test_data"}
	var result struct {
		Status string `json:"status"`
	}

	err := client.PostMethod(context.Background(), "/v1/test_post_endpoint", requestBody, &result)
	if err != nil {
		t.Fatalf("PostMethod failed: %v", err)
	}

	if result.Status != "posted" {
		t.Errorf("Expected status 'posted', got '%s'", result.Status)
	}

	calls := logger.getCalls()
	if len(calls) == 0 {
		t.Fatal("Expected logger to be called for PostMethod, but it wasn't")
	}

	expectedLog := fmt.Sprintf("POST %s/v1/test_post_endpoint", server.URL)
	foundLog := false
	for _, call := range calls {
		if strings.Contains(call, expectedLog) {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Errorf("Expected log message containing '%s', got calls: %v", expectedLog, calls)
	}
}

// Test for WithTimeout option (already existed implicitly but good to have an explicit one)
func TestClient_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Sleep longer than timeout
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	client := newClientInternal(server.URL, WithTimeout(50*time.Millisecond))
	var result struct{ Status string `json:"status"` }

	err := client.GetMethod(context.Background(), "/v1/timeout_test", &result)
	if err == nil {
		t.Fatal("Expected GetMethod to time out, but it didn't")
	}

	// Check if the error is a timeout error (net/http specific)
	// This can be fragile. A better check might be if the error is context.DeadlineExceeded
	// if the http client respects the context's deadline.
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "Timeout exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// Test for WithHTTPClient option
func TestClient_WithHTTPClient(t *testing.T) {
	customTransport := &http.Transport{
		ResponseHeaderTimeout: 5 * time.Second, // Custom property
	}
	customHttpClient := &http.Client{
		Transport: customTransport,
		Timeout:   10 * time.Second, // Different from default
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	}))
	defer server.Close()
	
	client := newClientInternal(server.URL, WithHTTPClient(customHttpClient))

	if client.httpclient.Timeout != 10*time.Second {
		t.Errorf("Expected client_test httpclient timeout to be %v, got %v", 10*time.Second, client.httpclient.Timeout)
	}
	
	// Check if a custom property of the transport is there (a bit of a deep check)
	if transport, ok := client.httpclient.Transport.(*http.Transport); ok {
		if transport.ResponseHeaderTimeout != 5*time.Second {
			t.Errorf("Expected client_test transport ResponseHeaderTimeout to be %v, got %v", 5*time.Second, transport.ResponseHeaderTimeout)
		}
	} else {
		t.Error("Client's httpclient is not using the expected *http.Transport type")
	}

	var result struct{ Status string `json:"status"` }
	err := client.GetMethod(context.Background(), "/v1/custom_client_test", &result)
	if err != nil {
		t.Fatalf("GetMethod with custom client_test failed: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result.Status)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client.baseHost != apiBaseHost {
		t.Errorf("NewClient() baseHost = %s; want %s", client.baseHost, apiBaseHost)
	}
	if client.httpclient == nil {
		t.Error("NewClient() httpclient is nil")
	}
}

func TestNewTestClient(t *testing.T) {
	client := NewTestClient()
	if client.baseHost != apiBaseHostTest {
		t.Errorf("NewTestClient() baseHost = %s; want %s", client.baseHost, apiBaseHostTest)
	}
	if client.httpclient == nil {
		t.Error("NewTestClient() httpclient is nil")
	}
}

func TestNewClientWithOpts(t *testing.T) {
	logger := &mockLogger{}
	timeout := 15 * time.Second
	client := NewClientWithOpts(WithLogger(logger), WithTimeout(timeout))

	if client.baseHost != apiBaseHost {
		t.Errorf("NewClientWithOpts() baseHost = %s; want %s", client.baseHost, apiBaseHost)
	}
	if client.logger != logger {
		t.Error("NewClientWithOpts() logger not set correctly")
	}
	if client.httpclient.Timeout != timeout {
		t.Errorf("NewClientWithOpts() timeout = %v; want %v", client.httpclient.Timeout, timeout)
	}
}

func TestNewTestClientWithOpts(t *testing.T) {
	logger := &mockLogger{}
	timeout := 18 * time.Second
	client := NewTestClientWithOpts(WithLogger(logger), WithTimeout(timeout))

	if client.baseHost != apiBaseHostTest {
		t.Errorf("NewTestClientWithOpts() baseHost = %s; want %s", client.baseHost, apiBaseHostTest)
	}
	if client.logger != logger {
		t.Error("NewTestClientWithOpts() logger not set correctly")
	}
	if client.httpclient.Timeout != timeout {
		t.Errorf("NewTestClientWithOpts() timeout = %v; want %v", client.httpclient.Timeout, timeout)
	}
}
