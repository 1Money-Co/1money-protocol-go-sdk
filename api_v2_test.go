package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

const testPrivateKey = "0x01833a126ec45d0191519748146b9e35647aab7fed28de1c8e17824970f964a3"

func testSigner(t *testing.T) Signer {
	t.Helper()
	s, err := NewPrivateKeySigner(testPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testPaymentPayload() PaymentPayload {
	return PaymentPayload{
		ChainID:   1212101,
		Nonce:     0,
		Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
		Value:     big.NewInt(10),
		Token:     common.HexToAddress("0x5458747a0efb9ebeb8696fcac1479278c0872fbe"),
	}
}

func batchPaymentFixture() BatchPaymentPayload {
	return BatchPaymentPayload{
		ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
		Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1)}},
		CreatedAt:  1,
	}
}

func paymentOp(p PaymentPayload) nativeV2Op {
	return nativeV2Op{
		op:          opPayment,
		payloadList: p.rlpList(),
		fields:      p.wireFields(),
		pathV1:      "/v1/transactions/payment",
		pathV2:      "/v2/transactions/payment",
	}
}

// -------- network-free request-assembly tests (always run) --------

func authorizedPayment(t *testing.T, opts ...SubmitOption) *AuthorizedTransaction {
	t.Helper()
	prep, err := PrepareTransaction(testPaymentPayload(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := prep.Authorize(sig)
	if err != nil {
		t.Fatal(err)
	}
	return authorized
}

func TestAuthorizedTransactionBodyShape(t *testing.T) {
	authorized := authorizedPayment(t)
	body := authorized.body
	// value must be a decimal string.
	if v, ok := body["value"].(string); !ok || v != "10" {
		t.Errorf("value = %v, want string \"10\"", body["value"])
	}
	// authorization must be the tagged single-sig union.
	authz, ok := body["authorization"].(nativeAuthorization)
	if !ok {
		t.Fatalf("authorization type = %T", body["authorization"])
	}
	if authz.Type != "single_secp256k1" {
		t.Errorf("authorization.type = %s", authz.Type)
	}
	if authz.Signature == nil || authz.Signature.V > 1 {
		t.Errorf("authorization.signature invalid: %+v", authz.Signature)
	}
	// memo present, no legacy top-level signature.
	if _, ok := body["memo"]; !ok {
		t.Error("v2 body missing memo")
	}
	if _, ok := body["signature"]; ok {
		t.Error("v2 body must not carry a top-level signature")
	}
	if authorized.path != "/v2/transactions/payment" {
		t.Errorf("path = %s, want /v2/transactions/payment", authorized.path)
	}
	if len(authorized.TransactionHash()) != 32 {
		t.Errorf("tx hash length = %d, want 32", len(authorized.TransactionHash()))
	}
	if _, err := json.Marshal(body); err != nil {
		t.Errorf("body not JSON-serializable: %v", err)
	}
}

func TestAuthorizedTransactionJSONWireShape(t *testing.T) {
	raw, err := json.Marshal(authorizedPayment(t).body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded["value"]) != `"10"` {
		t.Errorf("json value = %s, want \"10\"", decoded["value"])
	}
	var authz struct {
		Type      string `json:"type"`
		Signature struct {
			R string `json:"r"`
			S string `json:"s"`
			V uint64 `json:"v"`
		} `json:"signature"`
	}
	if err := json.Unmarshal(decoded["authorization"], &authz); err != nil {
		t.Fatal(err)
	}
	if authz.Type != "single_secp256k1" || authz.Signature.R == "" || authz.Signature.V > 1 {
		t.Errorf("authorization json wrong: %+v", authz)
	}
	var memo Memo
	if err := json.Unmarshal(decoded["memo"], &memo); err != nil {
		t.Fatalf("memo json: %v", err)
	}
}

func TestPrepareMemoChangesHash(t *testing.T) {
	memo := Memo{Type: "purpose/SALA", Format: "text/plain", Data: "invoice-0001"}
	pEmpty, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	pMemo, err := PrepareTransaction(testPaymentPayload(), WithMemo(memo))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(pEmpty.SigningHash(), pMemo.SigningHash()) {
		t.Error("memo must change the signing hash")
	}
	if got := authorizedPayment(t, WithMemo(memo)).body["memo"].(Memo); got != memo {
		t.Errorf("memo not carried into body: %+v", got)
	}
}

func TestBuildLegacyV1Body_Shape(t *testing.T) {
	body, err := buildLegacyV1Body(paymentOp(testPaymentPayload()), testSigner(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["signature"].(Signature); !ok {
		t.Errorf("v1 body signature type = %T", body["signature"])
	}
	if _, ok := body["authorization"]; ok {
		t.Error("v1 body must not carry an authorization union")
	}
	if _, ok := body["memo"]; ok {
		t.Error("v1 body must not carry a memo object")
	}
	if v, ok := body["value"].(string); !ok || v != "10" {
		t.Errorf("value = %v, want string \"10\"", body["value"])
	}
}

func TestCreateMultisigRejectsLegacyV1(t *testing.T) {
	c := NewClientWithCustomUrl("http://127.0.0.1:0", WithLegacyV1())
	if _, err := c.Accounts().CreateMultisig(context.Background(), CreateMultiSigPayload{ChainID: 1, Nonce: 0, Threshold: 1}, testSigner(t)); err == nil {
		t.Fatal("expected error for multisig creation under LegacyV1")
	}
}

// TestNamespaceRejectsNilSigner asserts a nil Signer yields a clear error rather
// than panicking inside the signing call — a public API must not crash the
// caller's process on a nil argument. The guard fires before any network I/O.
func TestNamespaceRejectsNilSigner(t *testing.T) {
	c := NewClient()
	if _, err := c.Transactions().Payment(context.Background(), testPaymentPayload(), nil); err == nil {
		t.Fatal("expected error for nil signer, got nil")
	}
	// Also covered on the legacy path and other namespaces (same funnel).
	if _, err := c.Tokens().Mint(context.Background(), TokenMintPayload{ChainID: 1, Nonce: 1, Recipient: repeatAddr(0x02), Value: big.NewInt(1), Token: repeatAddr(0x01)}, nil); err == nil {
		t.Fatal("expected error for nil signer on Tokens().Mint, got nil")
	}
}

// TestLegacyModeRejectsMemo asserts that an explicit memo is rejected (not
// silently dropped) when the client is in legacy v1 mode, which has no memo.
func TestLegacyModeRejectsMemo(t *testing.T) {
	c := NewClientWithOpts(WithLegacyV1())
	_, err := c.Transactions().Payment(context.Background(), testPaymentPayload(), testSigner(t), WithMemo(Memo{Type: "purpose/SALA"}))
	if err == nil {
		t.Fatal("expected memo-not-supported error in legacy v1 mode, got nil")
	}
}

// TestBatchPaymentSubmitsMemo asserts the batch v2 body always carries the
// three-field memo object, which the L1 BatchPaymentRequestV2 requires.
func TestBatchPaymentSubmitsMemo(t *testing.T) {
	memo := Memo{Type: "purpose/PAYROLL", Format: "text/plain", Data: "may-2026"}
	var gotMemo json.RawMessage
	var gotPath string
	hc := fakeHTTPClient(nil, func(path string, body map[string]json.RawMessage) interface{} {
		gotPath, gotMemo = path, body["memo"]
		return map[string]string{"hash": v2HashFromBody(body, opBatchPayment, batchPaymentFixture().rlpList(), memo)}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	if _, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t), WithMemo(memo)); err != nil {
		t.Fatalf("batch payment with memo should succeed: %v", err)
	}
	if gotPath != "/v2/transactions/batch_payment" {
		t.Errorf("path = %q, want /v2/transactions/batch_payment", gotPath)
	}
	if string(gotMemo) != `{"type":"purpose/PAYROLL","format":"text/plain","data":"may-2026"}` {
		t.Errorf("memo = %s, want the full three-field object", gotMemo)
	}
}

// TestMemoAcceptedOnV2MemoCapableOp guards the happy path: a memo on a
// memo-capable v2 operation must still succeed and reach the wire.
func TestMemoAcceptedOnV2MemoCapableOp(t *testing.T) {
	memo := Memo{Type: "purpose/SALA", Format: "text/plain", Data: "invoice-1"}
	var hadMemo bool
	hc := fakeHTTPClient(nil, func(_ string, body map[string]json.RawMessage) interface{} {
		_, hadMemo = body["memo"]
		return map[string]string{"hash": v2HashFromBody(body, opPayment, testPaymentPayload().rlpList(), memo)}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	if _, err := c.Transactions().Payment(context.Background(), testPaymentPayload(), testSigner(t), WithMemo(memo)); err != nil {
		t.Fatalf("memo-capable payment with WithMemo should succeed: %v", err)
	}
	if !hadMemo {
		t.Error("submitted body did not carry the memo")
	}
}

// -------- In-memory HTTP round-trip tests (no socket; run by default) --------
//
// These cover the security-critical client logic — v1/v2 endpoint routing,
// request-body shape, and fail-closed response-hash verification — through an
// http.RoundTripper that answers in memory. No TCP listener is opened, so they
// run under the default `go test ./...` (not only the ENABLE_HTTP_CLIENT_TESTS
// socket path) and drive the exact code Client.httpclient.Do drives in prod.

// roundTripFunc adapts a plain function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeHTTPClient returns an *http.Client whose transport intercepts every
// request in memory: it records the request path into *gotPath (if non-nil),
// decodes the JSON body, and encodes whatever respBody handle returns as a 200
// JSON response. No socket is opened.
func fakeHTTPClient(gotPath *string, handle func(path string, body map[string]json.RawMessage) interface{}) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		var body map[string]json.RawMessage
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			_ = json.Unmarshal(raw, &body)
		}
		buf, err := json.Marshal(handle(r.URL.Path, body))
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(buf)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})}
}

// v2HashFromBody reconstructs the v2 transaction hash from the signature in a
// request body so a mock server can echo the hash the client verifies against.
func v2HashFromBody(body map[string]json.RawMessage, op nativeOperationType, payloadList []interface{}, memo Memo) string {
	var authz struct {
		Signature Signature `json:"signature"`
	}
	if err := json.Unmarshal(body["authorization"], &authz); err != nil {
		return ""
	}
	payloadRLP, err := encodeWithMemo(payloadList, memo)
	if err != nil {
		return ""
	}
	proof, err := singleProof(authz.Signature)
	if err != nil {
		return ""
	}
	hash, err := txHashV2(op, singleDescriptor(), payloadRLP, proof)
	if err != nil {
		return ""
	}
	return hexLower(hash)
}

func TestTransactionsPaymentRoundTripV2(t *testing.T) {
	var gotPath string
	hc := fakeHTTPClient(&gotPath, func(_ string, body map[string]json.RawMessage) interface{} {
		return map[string]string{"hash": v2HashFromBody(body, opPayment, testPaymentPayload().rlpList(), EmptyMemo())}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	resp, err := c.Transactions().Payment(context.Background(), testPaymentPayload(), testSigner(t))
	if err != nil {
		t.Fatalf("payment: %v", err)
	}
	if resp.Hash == "" {
		t.Fatal("empty response hash")
	}
	if gotPath != "/v2/transactions/payment" {
		t.Errorf("path = %s, want /v2/transactions/payment", gotPath)
	}
}

func TestTransactionsPaymentRoundTripLegacyV1(t *testing.T) {
	var gotPath string
	hc := fakeHTTPClient(&gotPath, func(_ string, _ map[string]json.RawMessage) interface{} {
		return map[string]string{"hash": "0xdeadbeef"}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc), WithLegacyV1())
	if _, err := c.Transactions().Payment(context.Background(), testPaymentPayload(), testSigner(t)); err != nil {
		t.Fatalf("legacy payment: %v", err)
	}
	if gotPath != "/v1/transactions/payment" {
		t.Errorf("path = %s, want /v1/transactions/payment", gotPath)
	}
}

func TestPaymentV2HashMismatchFailsClosed(t *testing.T) {
	hc := fakeHTTPClient(nil, func(_ string, _ map[string]json.RawMessage) interface{} {
		return map[string]string{"hash": "0x11"}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	if _, err := c.Transactions().Payment(context.Background(), testPaymentPayload(), testSigner(t)); err == nil {
		t.Fatal("expected hash-mismatch error, got nil")
	}
}

// TestOfflineAuthorizeAndSubmitRoundTrip exercises the full offline pipeline:
// PrepareTransaction -> (external) sign the SigningHash -> Authorize ->
// Client.Submit, sharing the same submission logic as the namespace API.
func TestOfflineAuthorizeAndSubmitRoundTrip(t *testing.T) {
	prep, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	// "External" signing step (could be an HSM / air-gapped machine).
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := prep.Authorize(sig)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := hexLower(authorized.TransactionHash())

	var gotPath string
	hc := fakeHTTPClient(&gotPath, func(_ string, _ map[string]json.RawMessage) interface{} {
		return map[string]string{"hash": wantHash}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	resp, err := c.Submit(context.Background(), authorized)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Hash != wantHash {
		t.Errorf("resp.Hash = %s, want %s", resp.Hash, wantHash)
	}
	if gotPath != "/v2/transactions/payment" {
		t.Errorf("path = %s, want /v2/transactions/payment", gotPath)
	}
}

func TestSubmitHashMismatchFailsClosed(t *testing.T) {
	prep, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := prep.Authorize(sig)
	if err != nil {
		t.Fatal(err)
	}
	hc := fakeHTTPClient(nil, func(_ string, _ map[string]json.RawMessage) interface{} {
		return map[string]string{"hash": "0x11"}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	if _, err := c.Submit(context.Background(), authorized); err == nil {
		t.Fatal("expected hash-mismatch error from Submit, got nil")
	}
}

// TestLegacyModeRejectsV2OnlyOperations pins the generic capability check: an
// operation with no v1 path fails before signing and before any HTTP request,
// with a stable v2-only error. BatchPayment has no namespace-level guard, so
// this is its only protection. CreateMultisig already has a namespace-level
// guard (accounts.go:97-99) that fires first with a more specific message; the
// generic check still covers it as defense-in-depth for callers that reach the
// unexported submit core directly.
func TestLegacyModeRejectsV2OnlyOperations(t *testing.T) {
	newLegacyClient := func(t *testing.T, requests *int) *Client {
		t.Helper()
		hc := fakeHTTPClient(nil, func(_ string, _ map[string]json.RawMessage) interface{} {
			*requests++
			return map[string]string{"hash": "0x00"}
		})
		return NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc), WithLegacyV1())
	}

	t.Run("batch payment default memo", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		_, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t))
		if err == nil || !strings.Contains(err.Error(), "batch payment requires domain-separated v2 submission mode") {
			t.Fatalf("err = %v, want the batch payment v2-only error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})

	t.Run("batch payment explicit memo", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		_, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t), WithMemo(Memo{Type: "purpose/SALA"}))
		if err == nil || !strings.Contains(err.Error(), "batch payment requires domain-separated v2 submission mode") {
			t.Fatalf("err = %v, want the v2-only error, not the generic legacy-memo error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})

	// CreateMultisig already has its own namespace-level v2-only guard at
	// accounts.go:97-99 with a more specific message, and it returns before
	// reaching submitPayload. That guard stays — this subtest pins its behavior
	// rather than replacing it.
	t.Run("create multisig namespace guard", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		payload := CreateMultiSigPayload{
			ChainID: 1, Nonce: 1,
			Signers:   []MultiSigSigner{{PublicKey: validPubkey(t, 2), Weight: 1}},
			Threshold: 1,
		}
		_, err := c.Accounts().CreateMultisig(context.Background(), payload, testSigner(t))
		if err == nil || !strings.Contains(err.Error(), "has no legacy v1 endpoint") {
			t.Fatalf("err = %v, want the existing multisig v2-only error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})

	// The generic capability check is what protects a v2-only operation that has
	// no namespace-level guard. Exercise it through the unexported submit core,
	// which is the only path that reaches it for CreateMultisig.
	t.Run("create multisig generic capability check", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		payload := CreateMultiSigPayload{
			ChainID: 1, Nonce: 1,
			Signers:   []MultiSigSigner{{PublicKey: validPubkey(t, 2), Weight: 1}},
			Threshold: 1,
		}
		out := new(CreateMultisigResponse)
		err := c.submitPayload(context.Background(), payload, resolveSubmit(nil), testSigner(t), out)
		if err == nil || !strings.Contains(err.Error(), "create multisig requires domain-separated v2 submission mode") {
			t.Fatalf("err = %v, want the generic v2-only capability error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})
}

// TestLegacyModeResolvesOperationBeforeMemoGuard documents the intentional
// error-precedence change from moving operation resolution ahead of the legacy
// memo guard: an ambiguous v1-capable operation now reports the more specific
// resolution error rather than the generic legacy-memo error.
//
// This goes through the unexported submitPayload rather than a namespace method
// because Tokens().ManageBlacklist / ManageWhitelist always inject cfg.listKind
// (tokens.go:196-199, 206-209), so an ambiguous TokenManageListPayload is
// unreachable through the public namespace API.
func TestLegacyModeResolvesOperationBeforeMemoGuard(t *testing.T) {
	requests := 0
	hc := fakeHTTPClient(nil, func(_ string, _ map[string]json.RawMessage) interface{} {
		requests++
		return map[string]string{"hash": "0x00"}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc), WithLegacyV1())
	payload := TokenManageListPayload{ChainID: 1, Nonce: 1, Action: ManageListActionAdd, Address: repeatAddr(0x06), Token: repeatAddr(0x01)}
	// No listKind, plus an explicit memo: resolution must fail first.
	cfg := resolveSubmit([]SubmitOption{WithMemo(Memo{Type: "purpose/SALA"})})
	out := new(SetTokenManageListResponse)
	err := c.submitPayload(context.Background(), payload, cfg, testSigner(t), out)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v, want the ambiguous-operation error before the legacy-memo error", err)
	}
	if requests != 0 {
		t.Errorf("issued %d HTTP requests, want 0", requests)
	}
}
