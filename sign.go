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

func EncodePayload(payload interface{}) ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return nil, fmt.Errorf("rlp encode payload failed: %w", err)
	}
	return encoded, nil
}

// HashMessage encodes the message via RLP and returns the Keccak256 hash.
func HashMessage(msg interface{}) ([]byte, error) {
	encoded, err := EncodePayload(msg)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(encoded), nil
}

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
