package transactions

import (
	"fmt"
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
	hash := "0x819d23cce781550d03f47881606a59554ef0d87ea31e462bd51148b412052a1d"

	result, err := GetTransactionByHash(hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	// record the result for manual verification
	t.Logf("Transaction: %+v", result)
	t.Logf("Successfully retrieved transaction: %s", result.Hash)
	t.Logf("Transaction type: %s", result.TransactionType)

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
