package onemoney

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Hash computes the legacy v1 transaction hash for a signed payload.
//
// Deprecated: this is the pre-#1038 (non-domain-separated) hash and does not
// match domain-separated v2 transactions. For v2, use the hash returned by the
// submit response, or build it via PrepareTransaction(payload) →
// Authorize(sig) → AuthorizedTransaction.TransactionHash().
func Hash(payload interface{}, signature Signature) (common.Hash, error) {
	pEncode, err := EncodePayload(payload)
	if err != nil {
		return common.Hash{}, err
	}

	vEnc, err := encodeSignatureV(signature.V)
	if err != nil {
		return common.Hash{}, fmt.Errorf("%w", err)
	}

	rEnc, err := encodeSignatureRS(signature.R)
	if err != nil {
		return common.Hash{}, err
	}

	sEnc, err := encodeSignatureRS(signature.S)
	if err != nil {
		return common.Hash{}, err
	}

	vrsBytes := append(append(vEnc, rEnc...), sEnc...)

	totalLen := len(pEncode) + len(vrsBytes)
	header := encodeRLPListHeader(totalLen)

	txEnc := append(append(header, pEncode...), vrsBytes...)
	hash := crypto.Keccak256(txEnc)

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

func encodeSignatureV(value uint64) ([]byte, error) {
	var vBytes []byte
	if value == 0 {
		vBytes = []byte{}
	} else {
		vBytes = []byte{byte(value)}
	}
	return rlp.EncodeToBytes(vBytes)
}

func encodeSignatureRS(value string) ([]byte, error) {
	base := 10
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		value = value[2:]
		base = 16
	}
	component, ok := big.NewInt(0).SetString(value, base)
	if !ok {
		return nil, errors.New("invalid signature component: expected base-10 string or 0x-prefixed hex string")
	}
	return rlp.EncodeToBytes(component)
}
