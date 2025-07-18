package onemoney

import (
	"context"
	"fmt"
)

// GetCurrentEpochCheckpoint is a helper function that retrieves the current epoch and checkpoint
// information. This is commonly needed when constructing transaction payloads.
func (client *Client) GetCurrentEpochCheckpoint(ctx context.Context) (epoch uint64, checkpoint uint64, err error) {
	epochCheckpoint, err := client.GetLatestEpochCheckpoint(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get latest epoch checkpoint: %w", err)
	}
	return epochCheckpoint.Epoch, epochCheckpoint.Checkpoint, nil
}

// FillEpochCheckpoint is a helper function that automatically fills the RecentEpoch and
// RecentCheckpoint fields in various payload types. The payload must be a pointer to a struct
// that has RecentEpoch and RecentCheckpoint fields.
func (client *Client) FillEpochCheckpoint(ctx context.Context, payload interface{}) error {
	epoch, checkpoint, err := client.GetCurrentEpochCheckpoint(ctx)
	if err != nil {
		return err
	}

	// Use reflection to set the fields
	switch p := payload.(type) {
	case *PaymentPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *TokenIssuePayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *UpdateMetadataPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *TokenAuthorityPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *TokenMintPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *TokenBurnPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *TokenManageListPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	case *PauseTokenPayload:
		p.RecentEpoch = epoch
		p.RecentCheckpoint = checkpoint
	default:
		return fmt.Errorf("unsupported payload type: %T", payload)
	}

	return nil
}

