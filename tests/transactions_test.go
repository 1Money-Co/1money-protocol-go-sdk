package tests

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"go-1money/api"
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

	result, err := api.GetTransactionByHash(hash)
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
		if tokenData, ok := result.Data.(*api.TokenCreatePayload); ok {
			fmt.Printf("Token Symbol: %s\n", tokenData.Symbol)
		}
	case "TokenTransfer":
		if transferData, ok := result.Data.(*api.TokenTransferPayload); ok {
			fmt.Printf("Transfer Amount: %s\n", transferData.Value)
		}
	case "TokenMint":
		if mintData, ok := result.Data.(*api.TokenMintPayload); ok {
			fmt.Printf("Mint Amount: %s\n", mintData.Value)
		}
	}
	//TODO will add more types here
}

func TestGetTransactionReceipt(t *testing.T) {
	hash := "0x485667dd311b9ef9d966268672483246c2ffda4eeb52ea1ee59c1ed7cdeb407b"

	result, err := api.GetTransactionReceipt(hash)
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

	result, err := api.GetEstimateFee(from, token, value)
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

func TestSendPayment(t *testing.T) {
	// Get the current nonce
	var nonce uint64 = 0

	// Create payment payload
	payload := api.PaymentPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress("0x9E1E9688A44D058fF181Ed64ddFAFbBE5CC742Ab"),
		Value:     big.NewInt(100),
		Token:     common.HexToAddress("0x91f66cb6c9b56c7e3bcdb9eff9da13da171e89f4"),
	}

	// Sign the payload
	privateKey := strings.TrimPrefix(api.BurnAuthorityPrivateKey, "0x")
	signature, err := api.Message(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

	// Create payment request
	req := &api.PaymentRequest{
		PaymentPayload: payload,
		Signature: api.Signature{
			R: signature.R,
			S: signature.S,
			V: uint64(signature.V),
		},
	}

	// Send payment
	result, err := api.SendPayment(req)
	if err != nil {
		t.Fatalf("SendPayment failed: %v", err)
	}

	t.Log("\nPayment Result:")
	t.Log("==============")
	t.Logf("Transaction Hash: %s", result.Hash)
}
