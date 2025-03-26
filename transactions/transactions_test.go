package transactions

import (
	"fmt"
	"math/big"
	"testing"
)

func TestGetTransactionByHash(t *testing.T) {
	// use a known transaction hash for testing
	// tranaction type:
	// TokenCreate,
	// TokenTransfer,
	// TokenGrantAuthority,
	// TokenRevokeAuthority,
	// TokenBlacklistAccount,
	// TokenWhitelistAccount,
	// TokenMint,
	// TokenBurn,
	// TokenCloseAccount,
	// TokenPause,
	// TokenUnpause
	// for create/mint related transaction, can check cp=1 related transactions to get the hash to test
	hash := "0x485667dd311b9ef9d966268672483246c2ffda4eeb52ea1ee59c1ed7cdeb407b"

	result, err := GetTransactionByHash(hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}
	if result.TransactionType == "" {
		t.Error("Expected TransactionType to be present")
	}
	if result.From == "" {
		t.Error("Expected From to be present")
	}
	if result.ChainID == 0 {
		t.Error("Expected ChainID to be present")
	}

	t.Logf("Successfully retrieved transaction: %s", result.Hash)
	t.Logf("Transaction type: %s", result.TransactionType)
	t.Logf("From: %s", result.From)

	switch result.TransactionType {
	case "TokenCreate":
		if tokenData, ok := result.Data.(*TokenCreatePayload); ok {
			fmt.Printf("Token Symbol: %s\n", tokenData.Symbol)
		}
	case "TokenTransfer":
		if transferData, ok := result.Data.(*TokenTransferPayload); ok {
			fmt.Printf("Transfer Amount: %s\n", transferData.Value)
		}
	case "TokenMint":
		if mintData, ok := result.Data.(*TokenMintPayload); ok {
			fmt.Printf("Mint Amount: %s\n", mintData.Value)
		}
	}
	//TODO will add more types here
}

func TestGetTransactionReceipt(t *testing.T) {
	hash := "0x485667dd311b9ef9d966268672483246c2ffda4eeb52ea1ee59c1ed7cdeb407b"

	result, err := GetTransactionReceipt(hash)
	if err != nil {
		t.Fatalf("GetTransactionReceipt failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.TransactionHash == "" {
		t.Error("Expected TransactionHash to be present")
	}
	if result.From == "" {
		t.Error("Expected From to be present")
	}
	if result.To == "" {
		t.Error("Expected To to be present")
	}
	if result.CheckpointHash == "" {
		t.Error("Expected CheckpointHash to be present")
	}

	if result.CheckpointNumber <= 0 {
		t.Error("Expected CheckpointNumber to be positive")
	}
	if result.TransactionIndex < 0 {
		t.Error("Expected TransactionIndex to be non-negative")
	}
	if result.FeeUsed < 0 {
		t.Error("Expected FeeUsed to be non-negative")
	}

	if result.TransactionHash != hash {
		t.Errorf("Expected TransactionHash to be %s, got %s", hash, result.TransactionHash)
	}

	t.Logf("Successfully retrieved transaction receipt: %s", result.TransactionHash)
	t.Logf("From: %s", result.From)
	t.Logf("To: %s", result.To)
	t.Logf("Success: %v", result.Success)
	t.Logf("Fee Used: %d", result.FeeUsed)
}

func TestGetEstimateFee(t *testing.T) {
	from := "0x29b0fbe6aa3174ed8cc5900e2f3d81c765c116c6"
	token := "0x3c21b53619fdf08fbbe0615871a55fea79a9353b"
	value := "1500000" // 1 token with 18 decimals

	result, err := GetEstimateFee(from, token, value)
	if err != nil {
		t.Fatalf("GetEstimateFee failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Fee == "" {
		t.Error("Expected Fee to be present")
	}

	fee := new(big.Int)
	if _, ok := fee.SetString(result.Fee, 10); !ok {
		t.Error("Expected Fee to be a valid number")
	}

	if fee.Cmp(big.NewInt(0)) <= 0 {
		t.Error("Expected Fee to be positive")
	}

	t.Logf("Successfully estimated fee: %s", result.Fee)
}
