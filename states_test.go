package onemoney_test

import (
	"context"
	"testing"
	"time"

	onemoney "github.com/1Money-Co/1money-network-go-sdk"
)

const testTimeout = 30 * time.Second

func TestGetLatestEpochCheckpoint(t *testing.T) {
	client := onemoney.NewTestClient()
	result, err := client.GetLatestEpochCheckpoint(context.Background())
	if err != nil {
		t.Fatalf("GetLatestEpochCheckpoint failed: %v", err)
	}
	// Verify the result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	// Verify checkpoint hash is present
	if result.CheckpointHash == "" {
		t.Error("Expected CheckpointHash to be present")
	}
	// Verify checkpoint parent hash is present
	if result.CheckpointParentHash == "" {
		t.Error("Expected CheckpointParentHash to be present")
	}
	// Log the result for manual verification
	t.Logf("Successfully retrieved latest epoch checkpoint:")
	t.Logf("  Epoch: %d", result.Epoch)
	t.Logf("  Checkpoint: %d", result.Checkpoint)
	t.Logf("  CheckpointHash: %s", result.CheckpointHash)
	t.Logf("  CheckpointParentHash: %s", result.CheckpointParentHash)
}

func TestGetLatestEpochCheckpointWithContext(t *testing.T) {
	client := onemoney.NewTestClient()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	result, err := client.GetLatestEpochCheckpoint(ctx)
	if err != nil {
		t.Fatalf("GetLatestEpochCheckpoint with context failed: %v", err)
	}

	// Basic validation
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	// Ensure all fields are populated
	if result.Epoch == 0 || result.Checkpoint == 0 ||
		result.CheckpointHash == "" || result.CheckpointParentHash == "" {
		t.Error("Expected all fields to be populated")
	}
}

// TestGetLatestEpochCheckpointConsistency tests that consecutive calls return consistent results
func TestGetLatestEpochCheckpointConsistency(t *testing.T) {
	client := onemoney.NewTestClient()

	// First call
	result1, err := client.GetLatestEpochCheckpoint(context.Background())
	if err != nil {
		t.Fatalf("First GetLatestEpochCheckpoint call failed: %v", err)
	}

	// Second call
	result2, err := client.GetLatestEpochCheckpoint(context.Background())
	if err != nil {
		t.Fatalf("Second GetLatestEpochCheckpoint call failed: %v", err)
	}

	// The epoch should not decrease
	if result2.Epoch < result1.Epoch {
		t.Errorf("Epoch decreased between calls: %d -> %d", result1.Epoch, result2.Epoch)
	}

	// The checkpoint should not decrease within the same epoch
	if result2.Epoch == result1.Epoch && result2.Checkpoint < result1.Checkpoint {
		t.Errorf("Checkpoint decreased within same epoch: %d -> %d", result1.Checkpoint, result2.Checkpoint)
	}

	t.Logf("Consistency check passed. First: epoch=%d, checkpoint=%d; Second: epoch=%d, checkpoint=%d",
		result1.Epoch, result1.Checkpoint, result2.Epoch, result2.Checkpoint)
}
