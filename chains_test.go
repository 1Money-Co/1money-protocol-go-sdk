package onemoney_test

import (
	"context"
	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
	"testing"
)

func TestGetChainId(t *testing.T) {
	client := onemoney.NewTestClient()
	result, err := client.GetChainId(context.Background())
	if err != nil {
		t.Fatalf("GetChainId failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	t.Logf("chainId: %d", result.ChainId)
}
