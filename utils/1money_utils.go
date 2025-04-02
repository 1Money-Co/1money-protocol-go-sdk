package utils

import (
	"crypto/ecdsa"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// method to create a ethereum private key / public key pair
// return private key and public key
func TestCreateKeyPair(t *testing.T) {
	// 运行多次以验证随机性
	for i := 0; i < 3; i++ {
		privateKey, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		publicKey := crypto.FromECDSAPub(&privateKey.PublicKey)
		address := crypto.PubkeyToAddress(privateKey.PublicKey)

		fmt.Printf("=== Round %d ===\n", i+1)
		fmt.Println("Private Key:", common.BytesToHash(crypto.FromECDSA(privateKey)).Hex())
		fmt.Println("Public Key:", common.BytesToHash(publicKey).Hex())
		fmt.Println("Address:", address.Hex())
		fmt.Println()
	}
}

func TestRecoverAddressFromPrivateKey(t *testing.T) {
	// Test private key (remove "0x" prefix if present)
	privateKeyHex := "1a92ee8541aa114414b1a747a5072495cae1bc310012e26d098e554d4d50c951"

	// Convert hex string to ECDSA private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("Failed to convert hex to ECDSA: %v", err)
	}

	// Get public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("Failed to get public key")
	}

	// Get address
	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	fmt.Println("Private Key:", "0x"+privateKeyHex)
	fmt.Println("Address:", address.Hex())
}
