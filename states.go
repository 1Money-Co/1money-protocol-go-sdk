package onemoney

import (
	"context"
)

// EpochCheckpointResponse represents the current epoch and checkpoint information
type EpochCheckpointResponse struct {
	Epoch                uint64 `json:"epoch"`
	Checkpoint           uint64 `json:"checkpoint"`
	CheckpointHash       string `json:"checkpoint_hash"`
	CheckpointParentHash string `json:"checkpoint_parent_hash"`
}

// GetLatestEpochCheckpoint retrieves the current epoch and checkpoint information
func (client *Client) GetLatestEpochCheckpoint(ctx context.Context) (*EpochCheckpointResponse, error) {
	result := new(EpochCheckpointResponse)
	return result, client.GetMethod(ctx, "/v1/states/latest_epoch_checkpoint", result)
}
