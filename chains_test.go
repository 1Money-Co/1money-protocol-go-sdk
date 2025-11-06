//go:build integration

package onemoney

import (
	"context"
	"testing"
)

func TestGetChainId(t *testing.T) {
	client := NewTestClient()
	result, err := client.GetChainId(context.Background())
	if err != nil {
		t.Fatalf("GetChainId failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	t.Logf("chainId: %d", result.ChainId)
}
