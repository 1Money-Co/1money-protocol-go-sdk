package onemoney

import (
	"encoding/json"
	"testing"
)

// Regression test for the CreateMultisig public_key JSON encoding. The L1 DTO
// (CreateMultiSigPayload.signers[].public_key) is a bare Vec<u8>, which serde
// serializes as a JSON number array [2,17,...]. A Go []byte would marshal as a
// base64 string and a HexBytes as "0x...", either of which the server rejects
// at deserialization. This test asserts the wire body carries public_key as a
// number array of the 33 compressed-key bytes.
func TestCreateMultisigWireBodyPublicKeyIsNumberArray(t *testing.T) {
	pk := validPubkey(t, 2) // valid 33-byte SEC1-compressed key (passes config validation)
	payload := CreateMultiSigPayload{
		ChainID:   1212101,
		Nonce:     13,
		Signers:   []MultiSigSigner{{PublicKey: pk, Weight: 1}},
		Threshold: 1,
	}
	prep, err := PrepareTransaction(payload)
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
	raw, err := json.Marshal(authorized.body)
	if err != nil {
		t.Fatal(err)
	}

	// Decoding public_key into []int only succeeds if it is a JSON number array;
	// a hex string ("0x..") or base64 string would fail here.
	var decoded struct {
		Signers []struct {
			PublicKey []int `json:"public_key"`
			Weight    int   `json:"weight"`
		} `json:"signers"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("public_key is not a JSON number array: %v\nbody=%s", err, raw)
	}
	if len(decoded.Signers) != 1 {
		t.Fatalf("signers len = %d, want 1", len(decoded.Signers))
	}
	got := decoded.Signers[0].PublicKey
	if len(got) != 33 {
		t.Fatalf("public_key len = %d, want 33", len(got))
	}
	for i, b := range pk {
		if got[i] != int(b) {
			t.Fatalf("public_key[%d] = %d, want %d (byte array must match the pubkey)", i, got[i], b)
		}
	}
	if decoded.Signers[0].Weight != 1 {
		t.Errorf("weight = %d, want 1", decoded.Signers[0].Weight)
	}

	// Explicitly confirm the raw element is not string-encoded.
	var shape struct {
		Signers []struct {
			PublicKey json.RawMessage `json:"public_key"`
		} `json:"signers"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if b := shape.Signers[0].PublicKey; len(b) == 0 || b[0] != '[' {
		t.Errorf("public_key raw = %s, want a JSON array beginning with '['", b)
	}
}
