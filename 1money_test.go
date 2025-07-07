package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockLogger is a comprehensive mock for the Logger interface.
type mockLogger struct {
	t           *testing.T
	mu          sync.Mutex
	printfCalls []string
	infofCalls  []string
	warnfCalls  []string
	errorfCalls []string
}

func newMockLogger(t *testing.T) *mockLogger {
	return &mockLogger{t: t}
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.printfCalls = append(m.printfCalls, fmt.Sprintf(format, v...))
}

func (m *mockLogger) Infof(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infofCalls = append(m.infofCalls, fmt.Sprintf(format, v...))
}

func (m *mockLogger) Warnf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnfCalls = append(m.warnfCalls, fmt.Sprintf(format, v...))
}

func (m *mockLogger) Errorf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorfCalls = append(m.errorfCalls, fmt.Sprintf(format, v...))
}

func (m *mockLogger) getInfofCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]string, len(m.infofCalls))
	copy(calls, m.infofCalls)
	return calls
}

func (m *mockLogger) getErrorfCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]string, len(m.errorfCalls))
	copy(calls, m.errorfCalls)
	return calls
}

func (m *mockLogger) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.printfCalls = nil
	m.infofCalls = nil
	m.warnfCalls = nil
	m.errorfCalls = nil
}

// preRequestCall stores arguments for a PreRequest call.
type preRequestCall struct {
	ctx    context.Context
	method string
	url    string
	body   []byte
}

// postRequestCall stores arguments for a PostRequest call.
type postRequestCall struct {
	ctx          context.Context
	method       string
	url          string
	statusCode   int
	responseBody []byte
	err          error
}

type mockHook struct {
	t                *testing.T
	mu               sync.Mutex
	preRequestCalls  []preRequestCall
	postRequestCalls []postRequestCall
}

func newMockHook(t *testing.T) *mockHook {
	return &mockHook{t: t}
}

func (m *mockHook) PreRequest(ctx context.Context, method, url string, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var bodyCopy []byte
	if body != nil {
		bodyCopy = make([]byte, len(body))
		copy(bodyCopy, body)
	}
	m.preRequestCalls = append(m.preRequestCalls, preRequestCall{ctx, method, url, bodyCopy})
}

func (m *mockHook) PostRequest(ctx context.Context, method, url string, statusCode int, responseBody []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var bodyCopy []byte
	if responseBody != nil {
		bodyCopy = make([]byte, len(responseBody))
		copy(bodyCopy, responseBody)
	}
	m.postRequestCalls = append(m.postRequestCalls, postRequestCall{ctx, method, url, statusCode, bodyCopy, err})
}

func (m *mockHook) getPreRequestCalls() []preRequestCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]preRequestCall, len(m.preRequestCalls))
	copy(calls, m.preRequestCalls)
	return calls
}

func (m *mockHook) getPostRequestCalls() []postRequestCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]postRequestCall, len(m.postRequestCalls))
	copy(calls, m.postRequestCalls)
	return calls
}

func (m *mockHook) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preRequestCalls = nil
	m.postRequestCalls = nil
}

func TestPublicAddress(t *testing.T) {
	// 这是一个示例私钥（请勿在生产环境中使用！）
	// 你可以替换成你自己的私钥进行测试
	privateKeyExample := "0x76700ba1cb72480053d43b6202a16e9acbfb318b0321cfac4e55d38747bf9057" // 示例私钥

	address, err := PrivateKeyToAddress(privateKeyExample)
	if err != nil {
		log.Fatalf("生成地址失败: %v", err)
	}

	fmt.Printf("私钥: %s\n", privateKeyExample)
	fmt.Printf("派生出的以太坊地址: %s\n", address)
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

	logger := newMockLogger(t)
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

	// Using Infof calls now
	calls := logger.getInfofCalls()
	if len(calls) == 0 {
		t.Fatal("Expected logger.Infof to be called, but it wasn't")
	}

	expectedLog := fmt.Sprintf("GET %s/v1/test_endpoint", server.URL)
	foundLog := false
	for _, call := range calls {
		if strings.Contains(call, expectedLog) { // Check if the specific call is present
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Errorf("Expected Infof log message containing '%s', got calls: %v", expectedLog, calls)
	}

	// Test error logging (now using Errorf)
	logger.reset() // Reset all call logs in mockLogger
	err = client.GetMethod(context.Background(), "/v1/nonexistent_endpoint", &result)
	if err == nil {
		t.Fatal("Expected GetMethod to fail for nonexistent endpoint, but it didn't")
	}

	infofCalls := logger.getInfofCalls()
	errorfCalls := logger.getErrorfCalls()

	if len(infofCalls) != 1 { // Should have one Infof for the GET attempt
		t.Errorf("Expected 1 Infof call for error case, got: %d (%v)", len(infofCalls), infofCalls)
	}
	if len(errorfCalls) == 0 { // Should have at least one Errorf for the API error
		t.Fatalf("Expected logger.Errorf to be called for error case, got none. Infof: %v", infofCalls)
	}

	expectedErrorLog := fmt.Sprintf("API Error from GET %s/v1/nonexistent_endpoint: status=404", server.URL)
	foundErrorLog := false
	for _, call := range errorfCalls {
		if strings.Contains(call, expectedErrorLog) {
			foundErrorLog = true
			break
		}
	}
	if !foundErrorLog {
		t.Errorf("Expected Errorf log message containing '%s', got calls: %v", expectedErrorLog, errorfCalls)
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

	logger := newMockLogger(t)
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

	// Using Infof calls now
	calls := logger.getInfofCalls()
	if len(calls) == 0 {
		t.Fatal("Expected logger.Infof to be called for PostMethod, but it wasn't")
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
		t.Errorf("Expected Infof log message containing '%s', got calls: %v", expectedLog, calls)
	}
}

// Test for WithTimeout option
func TestClient_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Sleep longer than timeout
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	client := newClientInternal(server.URL, WithTimeout(50*time.Millisecond))
	var result struct {
		Status string `json:"status"`
	}

	err := client.GetMethod(context.Background(), "/v1/timeout_test", &result)
	if err == nil {
		t.Fatal("Expected GetMethod to time out, but it didn't")
	}

	// Check if the error is a timeout error (net/http specific)
	// if the http client_test respects the context's deadline.
	// The actual error message might vary slightly based on Go version or OS.
	// Checking for "context deadline exceeded" is generally robust.
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "Timeout exceeded") && !strings.Contains(err.Error(), "Client.Timeout exceeded") {
		t.Errorf("Expected timeout error containing 'context deadline exceeded' or 'Timeout exceeded', got: %v", err)
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
		t.Errorf("Expected client httpclient timeout to be %v, got %v", 10*time.Second, client.httpclient.Timeout)
	}

	// Check if a custom property of the transport is there
	if transport, ok := client.httpclient.Transport.(*http.Transport); ok {
		if transport.ResponseHeaderTimeout != 5*time.Second {
			t.Errorf("Expected client transport ResponseHeaderTimeout to be %v, got %v", 5*time.Second, transport.ResponseHeaderTimeout)
		}
	} else {
		t.Error("Client's httpclient is not using the expected *http.Transport type")
	}

	var result struct {
		Status string `json:"status"`
	}
	err := client.GetMethod(context.Background(), "/v1/custom_client_test", &result)
	if err != nil {
		t.Fatalf("GetMethod with custom client failed: %v", err)
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
	logger := newMockLogger(t)
	hook := newMockHook(t)
	timeout := 15 * time.Second
	client := NewClientWithOpts(WithLogger(logger), WithTimeout(timeout), WithHooks(hook))

	if client.baseHost != apiBaseHost {
		t.Errorf("NewClientWithOpts() baseHost = %s; want %s", client.baseHost, apiBaseHost)
	}
	if client.logger != logger {
		t.Error("NewClientWithOpts() logger not set correctly")
	}
	if client.httpclient.Timeout != timeout {
		t.Errorf("NewClientWithOpts() timeout = %v; want %v", client.httpclient.Timeout, timeout)
	}
	if len(client.hooks) != 1 || client.hooks[0] != hook {
		t.Error("NewClientWithOpts() hooks not set correctly")
	}
}

func TestNewTestClientWithOpts(t *testing.T) {
	logger := newMockLogger(t)
	hook := newMockHook(t)
	timeout := 18 * time.Second
	client := NewTestClientWithOpts(WithLogger(logger), WithTimeout(timeout), WithHooks(hook))

	if client.baseHost != apiBaseHostTest {
		t.Errorf("NewTestClientWithOpts() baseHost = %s; want %s", client.baseHost, apiBaseHostTest)
	}
	if client.logger != logger {
		t.Error("NewTestClientWithOpts() logger not set correctly")
	}
	if client.httpclient.Timeout != timeout {
		t.Errorf("NewTestClientWithOpts() timeout = %v; want %v", client.httpclient.Timeout, timeout)
	}
	if len(client.hooks) != 1 || client.hooks[0] != hook {
		t.Error("NewTestClientWithOpts() hooks not set correctly")
	}
}

// TestClientLoggingLevels tests the different logging levels used by the client.
func TestClientLoggingLevels(t *testing.T) {
	logger := newMockLogger(t)
	// var lastRequest *http.Request // To inspect the request in the handler, if needed

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// lastRequest = r // Save request
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"status":"all good"}`)
		case "/servererror":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{"error_code":"INTERNAL_ERROR","message":"Something broke on the server"}`)
		case "/decodeerror":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"status":"ok", "malformed_json":`) // Malformed JSON
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"error_code":"NOT_FOUND", "message":"Test endpoint not found"}`)
		}
	}))
	defer server.Close()

	client := newClientInternal(server.URL, WithLogger(logger), WithTimeout(1*time.Second))

	t.Run("Successful GET", func(t *testing.T) {
		logger.reset()
		var result struct{ Status string }
		err := client.GetMethod(context.Background(), "/success", &result)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result.Status != "all good" {
			t.Errorf("Expected status 'all good', got '%s'", result.Status)
		}

		infofCalls := logger.getInfofCalls()
		if len(infofCalls) != 1 {
			t.Errorf("Expected 1 Infof call, got %d: %v", len(infofCalls), infofCalls)
		} else if !strings.Contains(infofCalls[0], "GET "+server.URL+"/success") {
			t.Errorf("Expected Infof log for GET /success, got: %s", infofCalls[0])
		}
		if len(logger.getErrorfCalls()) > 0 {
			t.Errorf("Expected 0 Errorf calls, got %d: %v", len(logger.getErrorfCalls()), logger.getErrorfCalls())
		}
	})

	t.Run("Server-Side Error", func(t *testing.T) {
		logger.reset()
		var result interface{}
		err := client.GetMethod(context.Background(), "/servererror", &result)
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("Expected APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, apiErr.StatusCode)
		}

		infofCalls := logger.getInfofCalls()
		if len(infofCalls) != 1 {
			t.Errorf("Expected 1 Infof call for GET, got %d: %v", len(infofCalls), infofCalls)
		} else if !strings.Contains(infofCalls[0], "GET "+server.URL+"/servererror") {
			t.Errorf("Expected Infof log for GET /servererror, got: %s", infofCalls[0])
		}

		errorfCalls := logger.getErrorfCalls()
		if len(errorfCalls) != 1 {
			t.Errorf("Expected 1 Errorf call for API error, got %d: %v", len(errorfCalls), errorfCalls)
		} else if !strings.Contains(errorfCalls[0], "API Error from GET "+server.URL+"/servererror") || !strings.Contains(errorfCalls[0], "status=500") {
			t.Errorf("Expected Errorf log for API error on /servererror, got: %s", errorfCalls[0])
		}
	})

	t.Run("Response Decoding Error", func(t *testing.T) {
		logger.reset()
		var result interface{}
		err := client.GetMethod(context.Background(), "/decodeerror", &result)
		if err == nil {
			t.Fatal("Expected an error due to malformed JSON, got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode response") {
			t.Errorf("Expected error message to contain 'failed to decode response', got: %v", err)
		}

		infofCalls := logger.getInfofCalls()
		if len(infofCalls) != 1 {
			t.Errorf("Expected 1 Infof call for GET, got %d: %v", len(infofCalls), infofCalls)
		}

		errorfCalls := logger.getErrorfCalls()
		if len(errorfCalls) != 1 {
			t.Errorf("Expected 1 Errorf call for decode error, got %d: %v", len(errorfCalls), errorfCalls)
		} else if !strings.Contains(errorfCalls[0], "Failed to decode response from GET "+server.URL+"/decodeerror") {
			t.Errorf("Expected Errorf log for decoding error, got: %s", errorfCalls[0])
		}
	})

	t.Run("Client-Side Request Error (Timeout)", func(t *testing.T) {
		logger.reset()
		timeoutClient := newClientInternal(server.URL, WithLogger(logger), WithTimeout(1*time.Millisecond)) // very short timeout

		originalHandler := server.Config.Handler
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			originalHandler.ServeHTTP(w, r)
		})
		defer func() { server.Config.Handler = originalHandler }()

		var result interface{}
		err := timeoutClient.GetMethod(context.Background(), "/success", &result)
		if err == nil {
			t.Fatal("Expected an error due to timeout, got nil")
		}

		infofCalls := logger.getInfofCalls()
		if len(infofCalls) != 1 {
			t.Errorf("Expected 1 Infof call for GET, got %d: %v", len(infofCalls), infofCalls)
		}

		errorfCalls := logger.getErrorfCalls()
		if len(errorfCalls) != 1 {
			t.Errorf("Expected 1 Errorf call for the timeout, got %d: %v", len(errorfCalls), errorfCalls)
		} else if !strings.Contains(errorfCalls[0], "API GET request to "+server.URL+"/success failed") {
			t.Errorf("Expected Errorf log for API GET request failed, got: %s", errorfCalls[0])
		}
	})
}

func TestClientHooks(t *testing.T) {
	hook := newMockHook(t)
	// var lastRequest *http.Request // For server-side inspection if needed
	var requestBodyOnServer []byte // To store the body received by the server

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// lastRequest = r
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		requestBodyOnServer = body // Store it
		r.Body.Close()

		switch r.URL.Path {
		case "/get_ok":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"key":"value"}`)
		case "/post_ok":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"output":"response"}`)
		case "/api_error":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, `{"error_code":"BAD_INPUT","message":"Invalid input"}`)
		case "/unmarshal_error_path": // Specific path for unmarshal error test
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"key": "value", "extra": "malformed"`) // Malformed JSON
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"error_code":"NOT_FOUND", "message":"Test endpoint not found by hook test server"}`)
		}
	}))
	// server.Close() will be called after all sub-tests that use it are done.

	client := newClientInternal(server.URL, WithHooks(hook), WithTimeout(2*time.Second))

	t.Run("WithHooks option", func(t *testing.T) {
		if len(client.hooks) != 1 || client.hooks[0] != hook {
			t.Fatal("Client's hooks not set correctly by WithHooks")
		}
	})

	t.Run("Successful GET", func(t *testing.T) {
		hook.reset()
		requestBodyOnServer = nil
		var result struct{ Key string }
		err := client.GetMethod(context.Background(), "/get_ok", &result)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result.Key != "value" {
			t.Errorf("Expected key 'value', got '%s'", result.Key)
		}

		preCalls := hook.getPreRequestCalls()
		if len(preCalls) != 1 {
			t.Fatalf("Expected 1 PreRequest call, got %d", len(preCalls))
		}
		if preCalls[0].method != "GET" || !strings.HasSuffix(preCalls[0].url, "/get_ok") || preCalls[0].body != nil {
			t.Errorf("PreRequest call mismatch: %+v", preCalls[0])
		}

		postCalls := hook.getPostRequestCalls()
		if len(postCalls) != 1 {
			t.Fatalf("Expected 1 PostRequest call, got %d", len(postCalls))
		}
		expectedRespBody := []byte(`{"key":"value"}` + "\n")
		if postCalls[0].method != "GET" || !strings.HasSuffix(postCalls[0].url, "/get_ok") ||
			postCalls[0].statusCode != http.StatusOK || !bytes.Equal(postCalls[0].responseBody, expectedRespBody) || postCalls[0].err != nil {
			t.Errorf("PostRequest call mismatch: Method=%s URL=%s Status=%d Body=%s Err=%v. Expected body: %s",
				postCalls[0].method, postCalls[0].url, postCalls[0].statusCode, string(postCalls[0].responseBody), postCalls[0].err, string(expectedRespBody))
		}
	})

	t.Run("Successful POST", func(t *testing.T) {
		hook.reset()
		requestBodyOnServer = nil
		requestPayload := map[string]string{"input": "data"}
		expectedReqBodyBytes, _ := json.Marshal(requestPayload)
		var result struct{ Output string }

		err := client.PostMethod(context.Background(), "/post_ok", requestPayload, &result)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if result.Output != "response" {
			t.Errorf("Expected output 'response', got '%s'", result.Output)
		}

		// Check body received by server
		if !bytes.Equal(requestBodyOnServer, expectedReqBodyBytes) {
			t.Errorf("Server received body '%s', expected '%s'", string(requestBodyOnServer), string(expectedReqBodyBytes))
		}

		preCalls := hook.getPreRequestCalls()
		if len(preCalls) != 1 {
			t.Fatalf("Expected 1 PreRequest call, got %d", len(preCalls))
		}
		if preCalls[0].method != "POST" || !strings.HasSuffix(preCalls[0].url, "/post_ok") || !bytes.Equal(preCalls[0].body, expectedReqBodyBytes) {
			t.Errorf("PreRequest call mismatch: Method=%s URL=%s Body=%s, ExpectedBody=%s",
				preCalls[0].method, preCalls[0].url, string(preCalls[0].body), string(expectedReqBodyBytes))
		}

		postCalls := hook.getPostRequestCalls()
		if len(postCalls) != 1 {
			t.Fatalf("Expected 1 PostRequest call, got %d", len(postCalls))
		}
		expectedRespBody := []byte(`{"output":"response"}` + "\n")
		if postCalls[0].method != "POST" || !strings.HasSuffix(postCalls[0].url, "/post_ok") ||
			postCalls[0].statusCode != http.StatusOK || !bytes.Equal(postCalls[0].responseBody, expectedRespBody) || postCalls[0].err != nil {
			t.Errorf("PostRequest call mismatch: Method=%s URL=%s Status=%d Body=%s Err=%v. Expected body: %s",
				postCalls[0].method, postCalls[0].url, postCalls[0].statusCode, string(postCalls[0].responseBody), postCalls[0].err, string(expectedRespBody))
		}
	})

	t.Run("API Error", func(t *testing.T) {
		hook.reset()
		requestBodyOnServer = nil
		var result interface{}
		err := client.GetMethod(context.Background(), "/api_error", &result)
		if err == nil {
			t.Fatal("Expected an API error, got nil")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("Expected *APIError, got %T", err)
		}

		preCalls := hook.getPreRequestCalls()
		if len(preCalls) != 1 {
			t.Fatalf("Expected 1 PreRequest call, got %d", len(preCalls))
		}
		if preCalls[0].method != "GET" || !strings.HasSuffix(preCalls[0].url, "/api_error") {
			t.Errorf("PreRequest call mismatch: %+v", preCalls[0])
		}

		postCalls := hook.getPostRequestCalls()
		if len(postCalls) != 1 {
			t.Fatalf("Expected 1 PostRequest call, got %d", len(postCalls))
		}
		expectedErrBody := []byte(`{"error_code":"BAD_INPUT","message":"Invalid input"}` + "\n")
		if postCalls[0].method != "GET" || !strings.HasSuffix(postCalls[0].url, "/api_error") ||
			postCalls[0].statusCode != http.StatusBadRequest || !bytes.Equal(postCalls[0].responseBody, expectedErrBody) ||
			postCalls[0].err != apiErr {
			t.Errorf("PostRequest call mismatch: Method=%s URL=%s Status=%d Body=%s Err=%v. Expected body: %s, expected error: %v",
				postCalls[0].method, postCalls[0].url, postCalls[0].statusCode, string(postCalls[0].responseBody), postCalls[0].err, string(expectedErrBody), apiErr)
		}
	})

	// This sub-test must run before server.Close() if it relies on the main server.
	// For Response Unmarshal Error, we use the main server.
	t.Run("Response Unmarshal Error", func(t *testing.T) {
		hook.reset()
		requestBodyOnServer = nil
		var result struct{ Key string }
		err := client.GetMethod(context.Background(), "/unmarshal_error_path", &result)
		if err == nil {
			t.Fatal("Expected an unmarshal error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to decode response") {
			t.Errorf("Expected unmarshal error message, got: %v", err)
		}

		preCalls := hook.getPreRequestCalls()
		if len(preCalls) != 1 {
			t.Fatalf("Expected 1 PreRequest call, got %d", len(preCalls))
		}
		expectedURL := server.URL + "/unmarshal_error_path"
		if preCalls[0].method != "GET" || preCalls[0].url != expectedURL {
			t.Errorf("PreRequest call mismatch: %+v. Expected URL: %s", preCalls[0], expectedURL)
		}

		postCalls := hook.getPostRequestCalls()
		if len(postCalls) != 1 {
			t.Fatalf("Expected 1 PostRequest call for unmarshal error, got %d", len(postCalls))
		}

		malformedBody := []byte(`{"key": "value", "extra": "malformed"` + "\n")
		if postCalls[0].method != "GET" || postCalls[0].url != expectedURL ||
			postCalls[0].statusCode != http.StatusOK || !bytes.Equal(postCalls[0].responseBody, malformedBody) ||
			postCalls[0].err == nil {
			t.Errorf("PostRequest call mismatch for unmarshal error: Method=%s URL=%s Status=%d Body=%s Err=%v. Expected body: %s",
				postCalls[0].method, postCalls[0].url, postCalls[0].statusCode, string(postCalls[0].responseBody), postCalls[0].err, string(malformedBody))
		}
		// The exact error message for unmarshalling can vary slightly, check for common parts.
		if !strings.Contains(postCalls[0].err.Error(), "unexpected end of JSON input") && !strings.Contains(postCalls[0].err.Error(), "invalid character") && !strings.Contains(postCalls[0].err.Error(), "syntax error") {
			t.Errorf("PostRequest error content mismatch. Expected JSON unmarshal error, got: %v", postCalls[0].err)
		}
	})

	// Close the main server after tests that use it are done.
	server.Close()

	t.Run("Network Error", func(t *testing.T) {
		hook.reset()
		// Client pointing to a non-existent server
		networkErrorClient := newClientInternal("http://localhost:12345", WithHooks(hook), WithTimeout(100*time.Millisecond))
		var result interface{}
		err := networkErrorClient.GetMethod(context.Background(), "/some_path", &result)
		if err == nil {
			t.Fatal("Expected a network error, got nil")
		}
		if !strings.Contains(err.Error(), "refused") && !strings.Contains(err.Error(), "no such host") && !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("Expected network error (connection refused, no such host, or timeout), got: %v", err)
		}

		preCalls := hook.getPreRequestCalls()
		if len(preCalls) != 1 {
			t.Fatalf("Expected 1 PreRequest call, got %d", len(preCalls))
		}
		if preCalls[0].method != "GET" || preCalls[0].url != "http://localhost:12345/some_path" {
			t.Errorf("PreRequest call mismatch: %+v", preCalls[0])
		}

		postCalls := hook.getPostRequestCalls()
		if len(postCalls) != 1 {
			t.Fatalf("Expected 1 PostRequest call, got %d", len(postCalls))
		}
		if postCalls[0].method != "GET" || postCalls[0].url != "http://localhost:12345/some_path" ||
			postCalls[0].statusCode != 0 || postCalls[0].responseBody != nil || postCalls[0].err == nil {
			t.Errorf("PostRequest call mismatch for network error: %+v. Error was: %v", postCalls[0], err)
		}
		if postCalls[0].err != err {
			t.Errorf("PostRequest error mismatch. Expected: %v (%T), Got: %v (%T)", err, err, postCalls[0].err, postCalls[0].err)
		}
	})

	t.Run("Request Marshal Error", func(t *testing.T) {
		hook.reset()
		tempServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Server handler should not be called on marshal error")
			w.WriteHeader(http.StatusNoContent)
		}))
		defer tempServer.Close()
		marshalErrorClient := newClientInternal(tempServer.URL, WithHooks(hook), WithTimeout(1*time.Second))

		unmarshallableBody := make(chan int)
		var result interface{}
		err := marshalErrorClient.PostMethod(context.Background(), "/post_marshal_error", unmarshallableBody, &result)
		if err == nil {
			t.Fatal("Expected a marshal error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to marshal request") {
			t.Errorf("Expected marshal error message, got: %v", err)
		}

		preCalls := hook.getPreRequestCalls()
		if len(preCalls) != 0 {
			t.Fatalf("Expected 0 PreRequest calls, got %d: %+v", len(preCalls), preCalls)
		}

		postCalls := hook.getPostRequestCalls()
		if len(postCalls) != 1 {
			t.Fatalf("Expected 1 PostRequest call for marshal error, got %d", len(postCalls))
		}

		expectedURL := tempServer.URL + "/post_marshal_error"
		if postCalls[0].method != "POST" || postCalls[0].url != expectedURL ||
			postCalls[0].statusCode != 0 || postCalls[0].responseBody != nil || postCalls[0].err == nil {
			t.Errorf("PostRequest call mismatch for marshal error: %+v. Expected URL: %s. Error was: %v", postCalls[0], expectedURL, err)
		}
		if !strings.Contains(postCalls[0].err.Error(), "json: unsupported type: chan int") {
			t.Errorf("PostRequest error content mismatch. Expected JSON marshal error, got: %v", postCalls[0].err)
		}
		if postCalls[0].err != err { // Check if the error from PostMethod is the same as the one passed to the hook
			t.Errorf("PostRequest error instance mismatch. Expected: %p, Got: %p", err, postCalls[0].err)
		}
	})
}
