package onemoney

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// PrivateKeyToAddress converts a private key hex string to its corresponding Ethereum address.
// The privateKeyHex parameter can optionally include the "0x" prefix.
// Returns the address as a hex string with "0x" prefix, or an error if the private key is invalid.
func PrivateKeyToAddress(privateKeyHex string) (string, error) {
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %v", err)
	}

	publicKeyECDSA := &privateKey.PublicKey
	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	return address.Hex(), nil
}
