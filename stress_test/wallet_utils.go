package main

import (
	"crypto/ecdsa"
	"fmt"
	"os"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/crypto"
)

// generateWallet creates a new wallet with private key, public key, and address
func generateWallet() (*Wallet, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
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
