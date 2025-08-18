package onemoney_test

import (
	"context"
	onemoney "github.com/1Money-Co/1money-network-go-sdk"
	"testing"
)

func TestGetTokenAccount(t *testing.T) {
	client := onemoney.NewTestClient()
	address := onemoney.TestOperatorAddress
	token := onemoney.TestTokenAddress
	result, err := client.GetTokenAccount(context.Background(), address, token)
	if err != nil {
		t.Fatalf("GetTokenAccount failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.Balance == "" {
		t.Error("Expected Balance to be present")
	}
	if result.TokenAccountAddress == "" {
		t.Error("Expected TokenAccountAddress to be present")
	}
	t.Logf("Successfully retrieved token account: %s", result.TokenAccountAddress)
	t.Logf("Balance: %s", result.Balance)
	t.Logf("Nonce: %d", result.Nonce)
}

func TestGetAccountNonce(t *testing.T) {
	client := onemoney.NewTestClient()
	result, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("GetAccountNonce failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	t.Logf("Successfully retrieved account nonce: %d", result.Nonce)
}
