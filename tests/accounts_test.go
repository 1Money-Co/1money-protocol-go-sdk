package tests

import (
	"testing"

	"go-1money/api"
)

func TestGetTokenAccount(t *testing.T) {
	address := "0x29b0fbe6aa3174ed8cc5900e2f3d81c765c116c6"
	token := "0x3c21b53619fdf08fbbe0615871a55fea79a9353b"

	result, err := api.GetTokenAccount(address, token)
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
	address := "0x29b0fbe6aa3174ed8cc5900e2f3d81c765c116c6"

	result, err := api.GetAccountNonce(address)
	if err != nil {
		t.Fatalf("GetAccountNonce failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	t.Logf("Successfully retrieved account nonce: %d", result.Nonce)
}
