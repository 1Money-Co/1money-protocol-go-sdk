package utils

import (
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
