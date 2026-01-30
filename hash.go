package onemoney

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func Hash(payload interface{}, signature Signature) (common.Hash, error) {
	pEncode, err := EncodePayload(payload)
	if err != nil {
		return common.Hash{}, err
	}

	var vBytes []byte
	if signature.V == 0 {
		vBytes = []byte{}
	} else {
		vBytes = []byte{byte(signature.V)}
	}
	vEnc, err := rlp.EncodeToBytes(vBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("%w", err)
	}

	rBigInt, ok := big.NewInt(0).SetString(signature.R, 10)
	if !ok {
		return common.Hash{}, errors.New("")
	}
	rEnc, err := rlp.EncodeToBytes(rBigInt)

	sBigInt, ok := big.NewInt(0).SetString(signature.S, 10)
	if !ok {
		return common.Hash{}, errors.New("")
	}
	sEnc, err := rlp.EncodeToBytes(sBigInt)

	vrsBytes := append(append(vEnc, rEnc...), sEnc...)

	totalLen := len(pEncode) + len(vrsBytes)
	header := encodeRLPListHeader(totalLen)
	fullEncoded := append(append(header, pEncode...), vrsBytes...)

	hash := crypto.Keccak256(fullEncoded)
	return common.BytesToHash(hash), nil
}

// encodeRLPListHeader encodes an RLP list header for the given content length.
func encodeRLPListHeader(length int) []byte {
	if length < 56 {
		return []byte{byte(0xc0 + length)}
	}
	var lenBytes []byte
	temp := length
	for temp > 0 {
		lenBytes = append([]byte{byte(temp & 0xff)}, lenBytes...)
		temp >>= 8
	}
	return append([]byte{byte(0xf7 + len(lenBytes))}, lenBytes...)
}
