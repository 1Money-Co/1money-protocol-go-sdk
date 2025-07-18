package onemoney_test

import (
	"context"
	"testing"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

// TestGetCurrentEpochCheckpoint demonstrates the use of the helper function
func TestGetCurrentEpochCheckpoint(t *testing.T) {
	client := onemoney.NewTestClient()

	epoch, checkpoint, err := client.GetCurrentEpochCheckpoint(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentEpochCheckpoint failed: %v", err)
	}

	t.Logf("Current epoch: %d, checkpoint: %d", epoch, checkpoint)

	// Verify values are valid
	if epoch == 0 {
		t.Error("Expected epoch to be greater than 0")
	}
	if checkpoint == 0 {
		t.Error("Expected checkpoint to be greater than 0")
	}
}

// TestFillEpochCheckpoint demonstrates automatic filling of epoch/checkpoint fields
func TestFillEpochCheckpoint(t *testing.T) {
	client := onemoney.NewTestClient()

	// Test with PaymentPayload
	paymentPayload := &onemoney.PaymentPayload{
		ChainID:   1212101,
		Nonce:     1,
		Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
		Token:     common.HexToAddress(onemoney.TestTokenAddress),
	}

	err := client.FillEpochCheckpoint(context.Background(), paymentPayload)
	if err != nil {
		t.Fatalf("FillEpochCheckpoint failed for PaymentPayload: %v", err)
	}

	if paymentPayload.RecentEpoch == 0 {
		t.Error("Expected RecentEpoch to be filled")
	}
	if paymentPayload.RecentCheckpoint == 0 {
		t.Error("Expected RecentCheckpoint to be filled")
	}

	t.Logf("PaymentPayload filled with epoch: %d, checkpoint: %d",
		paymentPayload.RecentEpoch, paymentPayload.RecentCheckpoint)

	// Test with TokenMintPayload
	mintPayload := &onemoney.TokenMintPayload{
		ChainID:   1212101,
		Nonce:     1,
		Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
		Token:     common.HexToAddress(onemoney.TestTokenAddress),
	}

	err = client.FillEpochCheckpoint(context.Background(), mintPayload)
	if err != nil {
		t.Fatalf("FillEpochCheckpoint failed for TokenMintPayload: %v", err)
	}

	if mintPayload.RecentEpoch == 0 {
		t.Error("Expected RecentEpoch to be filled")
	}
	if mintPayload.RecentCheckpoint == 0 {
		t.Error("Expected RecentCheckpoint to be filled")
	}

	t.Logf("TokenMintPayload filled with epoch: %d, checkpoint: %d",
		mintPayload.RecentEpoch, mintPayload.RecentCheckpoint)
}

