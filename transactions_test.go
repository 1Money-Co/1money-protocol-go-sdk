package onemoney_test

import (
	"fmt"
	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
	"testing"
)

func TestGetTransactionByHash(t *testing.T) {
	client := onemoney.NewTestClient()
	// for create/mint related transaction, can check cp=1 related transactions to get the hash to test
	hash := "0x2bae78414bdf32bc6c8dd708eae72af2f5b4bf2dc649757cd0bfe499191dc277"
	result, err := client.GetTransactionByHash(hash)
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
		if tokenData, ok := result.Data.(*onemoney.TokenCreatePayload); ok {
			fmt.Printf("Token Symbol: %s\n", tokenData.Symbol)
		}
	case "TokenTransfer":
		if transferData, ok := result.Data.(*onemoney.TokenTransferPayload); ok {
			fmt.Printf("Transfer Amount: %s\n", transferData.Value)
		}
	case "TokenMint":
		if mintData, ok := result.Data.(*onemoney.TokenMintPayload); ok {
			fmt.Printf("Mint Amount: %s\n", mintData.Value)
		}
	}
	//TODO will add more types here
}

func TestGetTransactionReceipt(t *testing.T) {
	client := onemoney.NewTestClient()
	hash := "0x2bae78414bdf32bc6c8dd708eae72af2f5b4bf2dc649757cd0bfe499191dc277"
	result, err := client.GetTransactionReceipt(hash)
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
	client := onemoney.NewTestClient()
	from := "0xfcecaf244ce223050980038c4fe2328e7580afd9"
	token := "0x354312ce56a578c98559154Dd7A50F5C08D17270"
	value := "1500000" // 1 token with 18 decimals
	result, err := client.GetEstimateFee(from, token, value)
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
	client := onemoney.NewTestClient()
	var nonce uint64 = 11
	// Create payment payload
	payload := onemoney.PaymentPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress(onemoney.Test2ndAddress),
		Value:     big.NewInt(40250000),
		Token:     common.HexToAddress(onemoney.TestMintAccount),
	}
	// Sign the payload
	privateKey := strings.TrimPrefix(onemoney.TestOperatorPrivateKey, "0x")
	signature, err := client.SignMessage(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create payment request
	req := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send payment
	result, err := client.SendPayment(req)
	if err != nil {
		t.Fatalf("SendPayment failed: %v", err)
	}
	t.Log("\nPayment Result:")
	t.Log("==============")
	t.Logf("Transaction Hash: %s", result.Hash)
}
