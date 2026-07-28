package onemoney

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// validPubkey returns a distinct, valid 33-byte SEC1-compressed secp256k1 public
// key derived from a small deterministic seed (i must be > 0).
func validPubkey(t *testing.T, i int) []byte {
	t.Helper()
	var seed [32]byte
	seed[31] = byte(i)
	seed[30] = byte(i >> 8)
	key, err := crypto.ToECDSA(seed[:])
	if err != nil {
		t.Fatalf("bad seed %d: %v", i, err)
	}
	return crypto.CompressPubkey(&key.PublicKey)
}

// TestDeriveMultisigAddressVectors validates DeriveMultisigAddress against
// golden vectors generated from the L1 node's own canonical derivation
// (om-primitives derive_multisig_address, the execution path). This proves the
// Go-computed multisig account address is byte-for-byte identical to the address
// the server assigns.
func TestDeriveMultisigAddressVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/multisig-address-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Vectors []struct {
			Name      string `json:"name"`
			Address   string `json:"address"`
			Threshold uint16 `json:"threshold"`
			Signers   []struct {
				PublicKey string `json:"public_key"`
				Weight    uint8  `json:"weight"`
			} `json:"signers"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Vectors) == 0 {
		t.Fatal("no vectors loaded")
	}
	for _, v := range file.Vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			signers := make([]MultiSigSigner, len(v.Signers))
			for i, s := range v.Signers {
				signers[i] = MultiSigSigner{PublicKey: common.FromHex(s.PublicKey), Weight: s.Weight}
			}
			got, err := DeriveMultisigAddress(signers, v.Threshold)
			if err != nil {
				t.Fatalf("derive: %v", err)
			}
			if want := common.HexToAddress(v.Address); got != want {
				t.Fatalf("address = %s, want %s", got.Hex(), want.Hex())
			}
		})
	}
}

func TestDeriveMultisigAddressValidation(t *testing.T) {
	pk1 := validPubkey(t, 1)
	pk2 := validPubkey(t, 2)
	// 33 bytes but an invalid prefix / not a real curve point.
	badPubkey := append([]byte{0x00}, repeatBytes(0x11, 32)...)

	cases := []struct {
		name      string
		signers   []MultiSigSigner
		threshold uint16
	}{
		{"empty signers", nil, 1},
		{"zero threshold", []MultiSigSigner{{PublicKey: pk1, Weight: 1}}, 0},
		{"short pubkey", []MultiSigSigner{{PublicKey: []byte{0x02}, Weight: 1}}, 1},
		{"invalid pubkey", []MultiSigSigner{{PublicKey: badPubkey, Weight: 1}}, 1},
		{"zero weight", []MultiSigSigner{{PublicKey: pk1, Weight: 0}}, 1},
		{"duplicate pubkey", []MultiSigSigner{{PublicKey: pk1, Weight: 1}, {PublicKey: pk1, Weight: 1}}, 1},
		{"threshold exceeds total weight", []MultiSigSigner{{PublicKey: pk1, Weight: 1}}, 5},
	}
	for _, c := range cases {
		if _, err := DeriveMultisigAddress(c.signers, c.threshold); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}

	// u16 total-weight overflow: 258 distinct valid signers of weight 255 sum to
	// 65790 > 65535, which the node rejects (checked u16 addition).
	overflow := make([]MultiSigSigner, 0, 258)
	for i := 1; i <= 258; i++ {
		overflow = append(overflow, MultiSigSigner{PublicKey: validPubkey(t, i), Weight: 255})
	}
	if _, err := DeriveMultisigAddress(overflow, 1); err == nil {
		t.Error("expected total-weight overflow error")
	}

	// Address is independent of input order for a valid config.
	a1, err1 := DeriveMultisigAddress([]MultiSigSigner{{PublicKey: pk1, Weight: 1}, {PublicKey: pk2, Weight: 1}}, 2)
	a2, err2 := DeriveMultisigAddress([]MultiSigSigner{{PublicKey: pk2, Weight: 1}, {PublicKey: pk1, Weight: 1}}, 2)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v %v", err1, err2)
	}
	if a1 != a2 {
		t.Errorf("address must be order-independent: %s vs %s", a1.Hex(), a2.Hex())
	}
}
