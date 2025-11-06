//go:build integration

package onemoney

import (
	"context"
	"testing"
)

func TestGetCheckpointNumber(t *testing.T) {
	client := NewTestClient()
	result, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}
	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	// Verify the number is positive
	if result.Number <= 0 {
		t.Errorf("Expected number to be positive, got %d", result.Number)
	}
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint number: %d", result.Number)
}

func TestGetCheckpointByHashFull(t *testing.T) {
	client := NewTestClient()
	// First get a recent checkpoint to obtain a valid hash
	numResult, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}
	cpResult, err := client.GetCheckpointByNumber(context.Background(), numResult.Number-10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}
	hash := cpResult.Hash

	result, err := client.GetCheckpointByHash(context.Background(), hash, WithFullTransactions())
	if err != nil {
		t.Fatalf("GetCheckpointByHash with full transactions failed: %v", err)
	}
	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}
	if result.ParentHash == "" {
		t.Error("Expected ParentHash to be present")
	}
	// Verify full transactions are present
	if result.Transactions.Full == nil {
		t.Error("Expected full transactions to be present")
	}
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for hash: %s", hash)
	t.Logf("Size of transactions: %d", result.Size)
	t.Logf("Number of full transactions: %d", len(result.Transactions.Full))
	t.Log("result: ", result)
}

func TestGetCheckpointByHash(t *testing.T) {
	client := NewTestClient()
	// First get a recent checkpoint to obtain a valid hash
	numResult, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}
	cpResult, err := client.GetCheckpointByNumber(context.Background(), numResult.Number-10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}
	hash := cpResult.Hash

	result, err := client.GetCheckpointByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetCheckpointByHash failed: %v", err)
	}
	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}
	if result.ParentHash == "" {
		t.Error("Expected ParentHash to be present")
	}
	// Verify transaction hashes are present (not full transactions)
	if result.Transactions.Hashes == nil {
		t.Error("Expected transaction hashes to be present")
	}
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for hash: %s", hash)
	t.Logf("Size of transactions: %d", result.Size)
	t.Logf("Number of transaction hashes: %d", len(result.Transactions.Hashes))
	t.Log("result: ", result)
}

func TestGetCheckpointByNumberFull(t *testing.T) {
	client := NewTestClient()
	result, err := client.GetCheckpointByNumber(context.Background(), 99331, WithFullTransactions())
	if err != nil {
		t.Fatalf("GetCheckpointByNumber with full transactions failed: %v", err)
	}
	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}
	if result.ParentHash == "" {
		t.Error("Expected ParentHash to be present")
	}
	// Verify full transactions are present
	if result.Transactions.Full == nil {
		t.Error("Expected full transactions to be present")
	}
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for number: %d", result.Number)
	t.Logf("Size of transactions: %d", result.Size)
	t.Logf("Number of full transactions: %d", len(result.Transactions.Full))
	t.Log("result: ", result)
}

func TestGetCheckpointByNumber(t *testing.T) {
	client := NewTestClient()
	result, err := client.GetCheckpointByNumber(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}
	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}
	if result.ParentHash == "" {
		t.Error("Expected ParentHash to be present")
	}
	// Verify transaction hashes are present (not full transactions)
	if result.Transactions.Hashes == nil {
		t.Error("Expected transaction hashes to be present")
	}
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for number: %d", result.Number)
	t.Logf("Number of transactions: %d", result.Size)
	t.Logf("Number of transaction hashes: %d", len(result.Transactions.Hashes))
	t.Log("result: ", result)
}
