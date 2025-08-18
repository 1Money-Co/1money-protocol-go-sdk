package onemoney_test

import (
	"context"
	onemoney "github.com/1Money-Co/1money-network-go-sdk"
	"testing"
)

func TestGetCheckpointNumber(t *testing.T) {
	client := onemoney.NewTestClient()
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
	client := onemoney.NewTestClient()
	hash := "0xbdbbaa943cde023d600e2601fe7f2f8e13843e27392e03027b263ac386c1cfb5"
	result, err := client.GetCheckpointByHashFull(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetCheckpointByHashFull failed: %v", err)
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
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for hash: %s", hash)
	t.Logf("Number of transactions: %d", result.Size)
	t.Log("result: ", result)
}

func TestGetCheckpointByHash(t *testing.T) {
	client := onemoney.NewTestClient()
	hash := "0xbdbbaa943cde023d600e2601fe7f2f8e13843e27392e03027b263ac386c1cfb5"
	result, err := client.GetCheckpointByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetCheckpointByHashFull failed: %v", err)
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
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for hash: %s", hash)
	t.Logf("Number of transactions: %d", result.Size)
	t.Log("result: ", result)
}

func TestGetCheckpointByNumberFull(t *testing.T) {
	client := onemoney.NewTestClient()
	result, err := client.GetCheckpointByNumberFull(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumberFull failed: %v", err)
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
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for number: %d", result.Number)
	t.Logf("Number of transactions: %d", result.Size)
	t.Log("result: ", result)
}

func TestGetCheckpointByNumber(t *testing.T) {
	client := onemoney.NewTestClient()
	result, err := client.GetCheckpointByNumber(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumberFull failed: %v", err)
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
	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for number: %d", result.Number)
	t.Logf("Number of transactions: %d", result.Size)
	t.Log("result: ", result)
}
