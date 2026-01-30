package onemoney

import (
	"fmt"
	"math/big"
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

// HashMessage encodes the message via RLP and returns the Keccak256 hash.
func HashMessage(msg interface{}) ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(msg)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}
	return crypto.Keccak256(encoded), nil
}

type D struct {
	PaymentPayload
	V    bool
	R, S *big.Int
}

// Hash returns the RLP Keccak256 hash of a transaction request (payload + signature).
func Hash(payload interface{}, signature Signature) (common.Hash, error) {
	payloadBytes, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return common.Hash{}, fmt.Errorf("encode payload: %w", err)
	}
	// payloadContent, rest, err := rlp.SplitList(payloadBytes)
	// if err != nil {
	// 	return common.Hash{}, fmt.Errorf("split payload list: %w", err)
	// }
	// if len(rest) != 0 {
	// 	return common.Hash{}, fmt.Errorf("unexpected trailing bytes in payload encoding")
	// }

	rInt, err := parseSignatureScalar(signature.R)
	if err != nil {
		return common.Hash{}, err
	}
	sInt, err := parseSignatureScalar(signature.S)
	if err != nil {
		return common.Hash{}, err
	}

	vEnc, err := rlp.EncodeToBytes(signature.V == 1)
	if err != nil {
		return common.Hash{}, fmt.Errorf("encode signature v: %w", err)
	}
	rEnc, err := rlp.EncodeToBytes(rInt)
	if err != nil {
		return common.Hash{}, fmt.Errorf("encode signature r: %w", err)
	}
	sEnc, err := rlp.EncodeToBytes(sInt)
	if err != nil {
		return common.Hash{}, fmt.Errorf("encode signature s: %w", err)
	}

	var enc rlp.EncoderBuffer
	enc.Reset(nil)
	list := enc.List()
	enc.Write(payloadBytes)
	enc.Write(vEnc)
	enc.Write(rEnc)
	enc.Write(sEnc)
	enc.ListEnd(list)

	fmt.Println(common.Bytes2Hex(enc.ToBytes()))
	fmt.Println(enc.ToBytes())

	hash := crypto.Keccak256(enc.ToBytes())
	return common.BytesToHash(hash), nil
}

func parseSignatureScalar(value string) (*big.Int, error) {
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return new(big.Int), nil
	}
	out, ok := new(big.Int).SetString(trimmed, 16)
	if !ok {
		return nil, fmt.Errorf("invalid signature scalar: %q", value)
	}
	return out, nil
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
