package checkpoints

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
