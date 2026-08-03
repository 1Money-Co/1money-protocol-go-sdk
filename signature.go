package onemoney

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Signature struct {
	R string `json:"r"`
	S string `json:"s"`
	V uint64 `json:"v"`
}

// Signer abstracts transaction signing so the SDK never handles a raw private
// key directly in its high-level API. PrivateKeySigner is the built-in
// implementation; a KMS/HSM/MPC-backed signer can implement the same interface
// without any change to the submit methods.
//
// Custom implementations MUST honor this contract:
//
//   - SignHash signs the given 32-byte digest exactly as provided — no extra
//     hashing and no "\x19Ethereum Signed Message" prefix — and returns the raw
//     ECDSA components. The returned Signature.V MUST be the secp256k1 y-parity,
//     i.e. 0 or 1 — never the legacy Ethereum 27/28, an EIP-155 value, or
//     anything else. A non-0/1 v is rejected (Authorize returns an error),
//     because the node accepts only 0/1. r and s are the signature scalars,
//     carried as 0x-hex in Signature.
//   - CompressedPublicKey and Address MUST belong to the same key SignHash signs
//     with, so signer recovery matches on the node.
type Signer interface {
	// SignHash signs the 32-byte digest as-is and returns the ECDSA r, s and the
	// 0/1 y-parity v (never 27/28). See the Signer contract above.
	SignHash(hash []byte) (Signature, error)
	// CompressedPublicKey returns the 33-byte SEC1-compressed public key of the
	// signing key.
	CompressedPublicKey() []byte
	// Address returns the 20-byte account address of the signing key.
	Address() common.Address
}

type privateKeySigner struct {
	key *ecdsa.PrivateKey
}

// NewPrivateKeySigner builds a Signer from a hex-encoded secp256k1 private key
// (with or without a 0x prefix).
func NewPrivateKeySigner(hexKey string) (Signer, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(hexKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return &privateKeySigner{key: key}, nil
}

func (s *privateKeySigner) SignHash(hash []byte) (Signature, error) {
	sig, err := crypto.Sign(hash, s.key)
	if err != nil {
		return Signature{}, fmt.Errorf("sign hash: %w", err)
	}
	return Signature{
		R: common.BytesToHash(sig[:32]).Hex(),
		S: common.BytesToHash(sig[32:64]).Hex(),
		V: uint64(sig[64]),
	}, nil
}

func (s *privateKeySigner) CompressedPublicKey() []byte {
	return crypto.CompressPubkey(&s.key.PublicKey)
}

func (s *privateKeySigner) Address() common.Address {
	return crypto.PubkeyToAddress(s.key.PublicKey)
}
