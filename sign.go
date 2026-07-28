package onemoney

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type Signature struct {
	R string `json:"r"`
	S string `json:"s"`
	V uint64 `json:"v"`
}

// EncodePayload RLP-encodes a payload.
//
// Deprecated: v2 signing (domain separation) is handled internally by the
// Transactions()/Tokens()/Accounts() submit methods; this low-level legacy
// helper remains only for backward compatibility.
func EncodePayload(payload interface{}) ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return nil, fmt.Errorf("rlp encode payload failed: %w", err)
	}
	return encoded, nil
}

// HashMessage encodes the message via RLP and returns the Keccak256 hash.
//
// Deprecated: this is the legacy v1 (non-domain-separated) hash. New code uses
// the Transactions()/Tokens()/Accounts() submit methods, which compute the
// domain-separated v2 hash internally.
func HashMessage(msg interface{}) ([]byte, error) {
	encoded, err := EncodePayload(msg)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(encoded), nil
}

// SignMessage signs a payload with the legacy v1 scheme.
//
// Deprecated: signing is now handled internally by the Transactions()/Tokens()/
// Accounts() submit methods with a Signer, which use domain-separated v2
// signing by default. This method exposes the legacy scheme and remains only
// for backward compatibility.
func (client *Client) SignMessage(msg interface{}, privateKey string) (*Signature, error) {
	privateKey = strings.TrimPrefix(privateKey, "0x")
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	hash, err := HashMessage(msg)
	if err != nil {
		return nil, err
	}
	signature, err := crypto.Sign(hash, key)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	return &Signature{
		R: common.BytesToHash(signature[:32]).Hex(),
		S: common.BytesToHash(signature[32:64]).Hex(),
		V: uint64(signature[64]),
	}, nil
}
