package onemoney

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"
)

// TestPreparedTransactionMatchesGoldenVectors proves the public prepare API's
// SigningHash() is byte-for-byte identical to the L1 golden vectors, and that
// the transaction-hash encoding matches end to end. The golden fixtures pin the
// proof with a synthetic, non-canonical (high-S) signature, so the transaction
// hash is checked through the same internal hasher Authorize uses rather than
// the validating public Authorize; the public Authorize path with a valid
// low-S signature is covered by TestAuthorizeRejectsHighS and the round-trip tests.
func TestPreparedTransactionMatchesGoldenVectors(t *testing.T) {
	v, ok := vectorsByName(t)["Payment_single"]
	if !ok {
		t.Fatal("missing Payment_single vector")
	}
	// Canonical Payment fixture uses a populated memo (see the L1 fixture generator).
	memo := Memo{Type: "purpose/SALA", Format: "text/plain", Data: "invoice-0001"}
	payload := PaymentPayload{
		ChainID:   fixtureChainID,
		Nonce:     1,
		Recipient: repeatAddr(0x02),
		Value:     big.NewInt(1_000_000_000_000_000_000),
		Token:     repeatAddr(0x01),
	}
	prep, err := PrepareTransaction(payload, WithMemo(memo))
	if err != nil {
		t.Fatal(err)
	}
	if got := hexLower(prep.SigningHash()); got != v.SigningHash {
		t.Fatalf("SigningHash = %s, want %s", got, v.SigningHash)
	}

	var proof struct {
		Signature struct {
			R string `json:"r"`
			S string `json:"s"`
			V uint64 `json:"v"`
		} `json:"signature"`
	}
	if err := json.Unmarshal(v.AuthorizationProof, &proof); err != nil {
		t.Fatal(err)
	}
	// The fixture signature is synthetic and non-canonical (high-S), so it cannot
	// pass the validating public Authorize. Verify the identical transaction-hash
	// computation Authorize performs internally against the vector.
	proofSig := Signature{R: proof.Signature.R, S: proof.Signature.S, V: proof.Signature.V}
	txHash, err := txHashV2(prep.op, prep.descriptor, prep.payloadRLP, singleProof(proofSig))
	if err != nil {
		t.Fatal(err)
	}
	if got := hexLower(txHash); got != v.TransactionHash {
		t.Fatalf("TransactionHash = %s, want %s", got, v.TransactionHash)
	}
}

func TestPrepareManageListDisambiguation(t *testing.T) {
	payload := TokenManageListPayload{
		ChainID: 1212101, Nonce: 5, Action: ManageListActionAdd,
		Address: repeatAddr(0x06), Token: repeatAddr(0x01),
	}
	// Ambiguous without a kind.
	if _, err := PrepareTransaction(payload); err == nil {
		t.Fatal("expected error for TokenManageListPayload without WithManageListKind")
	}
	bl, err := PrepareTransaction(payload, WithManageListKind(ManageListBlacklist))
	if err != nil {
		t.Fatal(err)
	}
	wl, err := PrepareTransaction(payload, WithManageListKind(ManageListWhitelist))
	if err != nil {
		t.Fatal(err)
	}
	// Same payload bytes, different operation -> different signing hash (#1038).
	if bytes.Equal(bl.SigningHash(), wl.SigningHash()) {
		t.Error("blacklist and whitelist must produce different signing hashes")
	}
	// An unknown kind must error, not silently sign as blacklist: the operation
	// type is part of the signing domain, so there is no safe default.
	if _, err := PrepareTransaction(payload, WithManageListKind(ManageListKind(2))); err == nil {
		t.Error("expected error for invalid ManageListKind, got nil")
	}
}

func TestPrepareUnsupportedPayload(t *testing.T) {
	if _, err := PrepareTransaction(struct{ X int }{X: 1}); err == nil {
		t.Fatal("expected error for unsupported payload type")
	}
}

// TestPreparedConsistentWithSubmitPath asserts the public prepare API derives
// the same operation/signing hash the internal submit path uses for a payment.
func TestPreparedConsistentWithSubmitPath(t *testing.T) {
	payload := testPaymentPayload()
	prep, err := PrepareTransaction(payload)
	if err != nil {
		t.Fatal(err)
	}
	op := paymentOp(payload) // built exactly as TransactionsAPI.Payment does
	payloadRLP, err := encodeWithMemo(op.payloadList, EmptyMemo())
	if err != nil {
		t.Fatal(err)
	}
	want, err := signingHashV2(op.op, singleDescriptor(), payloadRLP)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prep.SigningHash(), want) {
		t.Errorf("prepare signing hash diverges from submit path")
	}
}

// TestAuthorizeRejectsNonParityV mirrors the node's strict parity rule: v must
// be 0 or 1; 2 / 27 / 28 / ... are rejected (never normalized).
func TestAuthorizeRejectsNonParityV(t *testing.T) {
	prep, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	for _, badV := range []uint64{2, 27, 28, 35, ^uint64(0)} {
		bad := sig
		bad.V = badV
		if _, err := prep.Authorize(bad); err == nil {
			t.Errorf("Authorize accepted invalid v=%d", badV)
		}
	}
	if _, err := prep.Authorize(sig); err != nil {
		t.Errorf("Authorize rejected a valid signature: %v", err)
	}
}

// TestAuthorizeRejectsHighS mirrors the node's canonical-low-S rule
// (CryptoError::HighSSignature): the built-in signer's low-S signature
// authorizes, but its high-S counterpart (s -> N-s) is rejected.
func TestAuthorizeRejectsHighS(t *testing.T) {
	prep, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Authorize(sig); err != nil {
		t.Fatalf("low-S signature rejected: %v", err)
	}
	// N - s is the high-S counterpart of a low-S s; the node rejects it, so
	// Authorize must too.
	s, ok := new(big.Int).SetString(sig.S[2:], 16)
	if !ok {
		t.Fatalf("could not parse sig.S %q", sig.S)
	}
	high := sig
	high.S = hexLower(new(big.Int).Sub(secp256k1N, s).Bytes())
	if _, err := prep.Authorize(high); err == nil {
		t.Error("Authorize accepted a high-S signature")
	}
}
