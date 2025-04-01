package api

import (
	"testing"
)

func TestGetCheckpointNumber(t *testing.T) {
	// Test the GetCheckpointNumber function
	result, err := GetCheckpointNumber()
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

func TestGetCheckpointByNumber(t *testing.T) {
	// Test with a known checkpoint number and full=false
	result, err := GetCheckpointByNumber(900, false)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}

	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	// Verify required fields are present
	if result.Number == "" {
		t.Error("Expected Number to be present")
	}
	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}
	if result.ParentHash == "" {
		t.Error("Expected ParentHash to be present")
	}
	if result.Timestamp == "" {
		t.Error("Expected Timestamp to be present")
	}

	// Verify size matches transactions length
	if result.Size != len(result.Transactions) {
		t.Errorf("Expected Size (%d) to match Transactions length (%d)", result.Size, len(result.Transactions))
	}

	// Log the result for manual verification
	t.Logf("Successfully retrieved checkpoint detail for number: %s", result.Number)
	t.Logf("Number of transactions: %d", result.Size)
}
