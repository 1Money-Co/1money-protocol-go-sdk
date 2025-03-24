package sign

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type Signature struct {
	R string
	S string
	V uint64
}

func Message(msg interface{}, privateKey string) (*Signature, error) {
	encoded, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	hash := crypto.Keccak256(encoded)
	fmt.Printf("Signature Hash: %s\n", common.BytesToHash(hash))
	signature, err := crypto.Sign(hash, key)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	fmt.Printf("Signature: 0x%x\n", signature)
	fmt.Printf("Raw bytes: %v\n", signature)
	return &Signature{
		R: common.BytesToHash(signature[:32]).Hex(),
		S: common.BytesToHash(signature[32:64]).Hex(),
		V: uint64(signature[64]),
	}, nil
}
