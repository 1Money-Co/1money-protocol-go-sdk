package onemoney

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
)

func batchPaymentFixture() BatchPaymentPayload {
	return BatchPaymentPayload{
		ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
		Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1)}},
		MaxFee:     big.NewInt(1), CreatedAt: 1,
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

// TestBatchPaymentRejectsMemo asserts that an explicit memo is rejected (not
// silently dropped) for batch payments, which carry no memo.
func TestBatchPaymentRejectsMemo(t *testing.T) {
	c := NewClient()
	_, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t), WithMemo(Memo{Type: "purpose/SALA"}))
	if err == nil {
		t.Fatal("expected memo-not-supported error for batch payment, got nil")
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
