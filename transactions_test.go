package onemoney

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Helper to create a client and server for testing transaction methods
func setupTransactionTest(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	// Use newClientInternal to ensure the test server's URL is used correctly.
	// Pass server.URL as baseHost
	client := newClientInternal(server.URL, WithTimeout(3*time.Second))
	return client, server
}

func TestGetTransactionByHash_ContextCancellation(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Ensure the request takes some time
		// Send a minimal valid Transaction JSON to avoid decode errors if request proceeds
		fmt.Fprintln(w, `{"hash":"test_hash_ctx", "transaction_type":"type", "chain_id":1, "checkpoint_hash":"chash", "checkpoint_number":1, "fee":1, "from":"from_addr", "nonce":1, "transaction_index":1}`)
	}
	client, server := setupTransactionTest(t, handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // Short timeout
	defer cancel()

	time.Sleep(50 * time.Millisecond) // Give context time to expire

	_, err := client.GetTransactionByHash(ctx, "somehash")
	if err == nil {
		t.Fatal("Expected an error due to context cancellation/timeout, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected error to contain 'context deadline exceeded' or 'context canceled', got: %v", err)
	}
}

func TestGetTransactionByHash_URLConstruction(t *testing.T) {
	var capturedURL *url.URL
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		fmt.Fprintln(w, `{"hash":"test_hash_url", "transaction_type":"type", "chain_id":1, "checkpoint_hash":"chash", "checkpoint_number":1, "fee":1, "from":"from_addr", "nonce":1, "transaction_index":1}`)
	}
	client, server := setupTransactionTest(t, handler)
	defer server.Close()

	hash := "0x123abcDEF" // Mixed case to ensure encoding handles it if necessary
	_, err := client.GetTransactionByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash failed: %v", err)
	}

	if capturedURL == nil {
		t.Fatal("capturedURL is nil, handler was likely not called")
	}

	expectedPath := "/v1/transactions/by_hash"
	if capturedURL.Path != expectedPath {
		t.Errorf("URL path mismatch: expected '%s', got '%s'", expectedPath, capturedURL.Path)
	}
	query := capturedURL.Query()
	if query.Get("hash") != hash {
		t.Errorf("URL query param 'hash' mismatch: expected '%s', got '%s'", hash, query.Get("hash"))
	}
}

func TestGetTransactionReceipt_URLConstruction(t *testing.T) {
	var capturedURL *url.URL
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		fmt.Fprintln(w, `{"transaction_hash":"receipt_hash_url", "checkpoint_hash":"chash", "checkpoint_number":1, "fee_used":1, "from":"from", "success":true, "to":"to", "token_address":"taddr", "transaction_index":1}`)
	}
	client, server := setupTransactionTest(t, handler)
	defer server.Close()

	hash := "0x456defGHI"
	_, err := client.GetTransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetTransactionReceipt failed: %v", err)
	}

	if capturedURL == nil {
		t.Fatal("capturedURL is nil, handler was likely not called")
	}

	expectedPath := "/v1/transactions/receipt/by_hash"
	if capturedURL.Path != expectedPath {
		t.Errorf("URL path mismatch: expected '%s', got '%s'", expectedPath, capturedURL.Path)
	}
	query := capturedURL.Query()
	if query.Get("hash") != hash {
		t.Errorf("URL query param 'hash' mismatch: expected '%s', got '%s'", hash, query.Get("hash"))
	}
}

func TestGetEstimateFee_URLConstruction(t *testing.T) {
	var capturedURL *url.URL
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		fmt.Fprintln(w, `{"fee":"1000"}`)
	}
	client, server := setupTransactionTest(t, handler)
	defer server.Close()

	from := "0xfromAddressJKL"
	token := "0xtokenAddressMNO"
	value := "1234567890"
	_, err := client.GetEstimateFee(context.Background(), from, token, value)
	if err != nil {
		t.Fatalf("GetEstimateFee failed: %v", err)
	}

	if capturedURL == nil {
		t.Fatal("capturedURL is nil, handler was likely not called")
	}

	expectedPath := "/v1/transactions/estimate_fee"
	if capturedURL.Path != expectedPath {
		t.Errorf("URL path mismatch: expected '%s', got '%s'", expectedPath, capturedURL.Path)
	}
	query := capturedURL.Query()
	if query.Get("from") != from {
		t.Errorf("URL query param 'from' mismatch: expected '%s', got '%s'", from, query.Get("from"))
	}
	if query.Get("token") != token {
		t.Errorf("URL query param 'token' mismatch: expected '%s', got '%s'", token, query.Get("token"))
	}
	if query.Get("value") != value {
		t.Errorf("URL query param 'value' mismatch: expected '%s', got '%s'", value, query.Get("value"))
	}
}

func TestSendPayment_ContextAndURLAndBody(t *testing.T) {
	var capturedURL *url.URL
	var capturedMethod string
	var capturedBodyBytes []byte

	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		capturedMethod = r.Method
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			t.Logf("Server failed to read request body: %v", err) // Log error in test context
			return
		}
		capturedBodyBytes = body
		r.Body.Close()
		fmt.Fprintln(w, `{"hash":"payment_hash_ok"}`)
	}
	client, server := setupTransactionTest(t, handler)
	defer server.Close()

	// 1. Test Context Cancellation
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancelTimeout()

	time.Sleep(50 * time.Millisecond) // Give context time to expire

	paymentReqMinimal := &PaymentRequest{
		PaymentPayload: PaymentPayload{ChainID: 0, Nonce: 0}, // Minimal valid
	}
	_, err := client.SendPayment(ctxTimeout, paymentReqMinimal)

	if err == nil {
		t.Fatal("Expected SendPayment to fail due to context timeout, but it succeeded")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context deadline or canceled error, got: %v", err)
	}

	// 2. Test URL, Method and Body Construction (with a fresh context)
	freshCtx := context.Background()
	expectedPayload := PaymentPayload{
		ChainID:   123,
		Nonce:     1,
		Recipient: common.HexToAddress("0xaaaabbbbccccddddeeeeffff0000111122223333"),
		Value:     big.NewInt(10000),
		Token:     common.HexToAddress("0x1111222233334444555566667777888899990000"),
	}
	paymentReqFull := &PaymentRequest{
		PaymentPayload: expectedPayload,
		Signature:      Signature{R: "test_r_value", S: "test_s_value", V: 27},
	}

	// Reset captured variables for this part of the test
	capturedURL = nil
	capturedMethod = ""
	capturedBodyBytes = nil

	_, err = client.SendPayment(freshCtx, paymentReqFull)
	if err != nil {
		t.Fatalf("SendPayment failed for URL/body test: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("HTTP method mismatch: expected 'POST', got '%s'", capturedMethod)
	}

	if capturedURL == nil {
		t.Fatal("capturedURL is nil, handler was likely not called for SendPayment freshCtx test")
	}
	expectedPath := "/v1/transactions/payment"
	if capturedURL.Path != expectedPath { // For POST requests, the path should be exact.
		t.Errorf("URL path mismatch: expected '%s', got '%s'", expectedPath, capturedURL.Path)
	}
	if capturedURL.RawQuery != "" {
		t.Errorf("URL query params mismatch: expected no query params, got '%s'", capturedURL.RawQuery)
	}


	var decodedBody PaymentRequest
	err = json.Unmarshal(capturedBodyBytes, &decodedBody)
	if err != nil {
		t.Fatalf("Failed to unmarshal captured request body: %v. Body: %s", err, string(capturedBodyBytes))
	}

	if decodedBody.ChainID != expectedPayload.ChainID {
		t.Errorf("Body ChainID mismatch: expected %d, got %d", expectedPayload.ChainID, decodedBody.ChainID)
	}
	if decodedBody.Nonce != expectedPayload.Nonce {
		t.Errorf("Body Nonce mismatch: expected %d, got %d", expectedPayload.Nonce, decodedBody.Nonce)
	}
	if decodedBody.Recipient != expectedPayload.Recipient {
		t.Errorf("Body Recipient mismatch: expected %s, got %s", expectedPayload.Recipient.Hex(), decodedBody.Recipient.Hex())
	}
	if decodedBody.Value.Cmp(expectedPayload.Value) != 0 {
		t.Errorf("Body Value mismatch: expected %s, got %s", expectedPayload.Value.String(), decodedBody.Value.String())
	}
	if decodedBody.Token != expectedPayload.Token {
		t.Errorf("Body Token mismatch: expected %s, got %s", expectedPayload.Token.Hex(), decodedBody.Token.Hex())
	}
	if decodedBody.Signature.R != paymentReqFull.Signature.R {
		t.Errorf("Body Signature.R mismatch: expected %s, got %s", paymentReqFull.Signature.R, decodedBody.Signature.R)
	}
	if decodedBody.Signature.S != paymentReqFull.Signature.S {
		t.Errorf("Body Signature.S mismatch: expected %s, got %s", paymentReqFull.Signature.S, decodedBody.Signature.S)
	}
	if decodedBody.Signature.V != paymentReqFull.Signature.V {
		t.Errorf("Body Signature.V mismatch: expected %d, got %d", paymentReqFull.Signature.V, decodedBody.Signature.V)
	}
}
