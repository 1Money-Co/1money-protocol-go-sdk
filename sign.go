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

func (client *Client) SignMessage(msg interface{}, privateKey string) (*Signature, error) {
	privateKey = strings.TrimPrefix(privateKey, "0x")
	encoded, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	hash := crypto.Keccak256(encoded)
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
