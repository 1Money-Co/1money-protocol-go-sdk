package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"os"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/crypto"
)

// Deterministic wallet generation constants
const (
	WALLET_SEED_BASE = "1money-stress-test-deterministic-seed"
)

// generateDeterministicPrivateKey generates a deterministic private key based on wallet type and index
func generateDeterministicPrivateKey(walletType string, index int) (*ecdsa.PrivateKey, error) {
	// Create a deterministic seed by combining base seed, wallet type, and index
	seedString := fmt.Sprintf("%s-%s-%d", WALLET_SEED_BASE, walletType, index)

	// Hash the seed to create a 32-byte private key
	hash := sha256.Sum256([]byte(seedString))

	// Create private key from the hash
	privateKey, err := crypto.ToECDSA(hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create private key from seed: %w", err)
	}

	return privateKey, nil
}

// generateDeterministicWallet creates a deterministic wallet based on wallet type and index
func generateDeterministicWallet(walletType string, index int) (*Wallet, error) {
	privateKey, err := generateDeterministicPrivateKey(walletType, index)
	if err != nil {
		return nil, fmt.Errorf("failed to generate deterministic private key: %w", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &Wallet{
		PrivateKey: fmt.Sprintf("%x", crypto.FromECDSA(privateKey)),
		PublicKey:  fmt.Sprintf("%x", crypto.FromECDSAPub(publicKeyECDSA)),
		Address:    address.Hex(),
	}, nil
}

// getOperatorConfig retrieves operator wallet configuration from environment variables or SDK defaults
func getOperatorConfig() (privateKey, address string, err error) {
	// Priority: use environment variables
	if pk := os.Getenv("OPERATOR_PRIVATE_KEY"); pk != "" {
		if addr := os.Getenv("OPERATOR_ADDRESS"); addr != "" {
			return pk, addr, nil
		}
	}

	// Fallback: use SDK test configuration (if available)
	if onemoney.TestOperatorPrivateKey != "" && onemoney.TestOperatorAddress != "" {
		return onemoney.TestOperatorPrivateKey, onemoney.TestOperatorAddress, nil
	}

	return "", "", fmt.Errorf("operator configuration not found. Please set OPERATOR_PRIVATE_KEY and OPERATOR_ADDRESS environment variables, or configure TestOperatorPrivateKey and TestOperatorAddress in the SDK")
}
