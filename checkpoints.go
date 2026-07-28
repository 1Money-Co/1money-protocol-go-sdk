package onemoney

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

const (
	endpointCheckpointsNumber           = "/v1/checkpoints/number"
	endpointCheckpointsByHash           = "/v1/checkpoints/by_hash"
	endpointCheckpointsByNumber         = "/v1/checkpoints/by_number"
	endpointCheckpointsReceiptsByNumber = "/v1/checkpoints/receipts/by_number"
)

// GetCheckpointNumber fetches the latest checkpoint number.
func (client *Client) GetCheckpointNumber(ctx context.Context) (*CheckpointNumber, error) {
	result := new(CheckpointNumber)
	return result, client.GetMethod(ctx, endpointCheckpointsNumber, result)
}

// GetCheckpointByHash retrieves a checkpoint by hash. By default only transaction hashes are returned.
// Use WithFullTransactions to fetch full transaction details.
func (client *Client) GetCheckpointByHash(ctx context.Context, hash string, opts ...CheckpointOption) (*Checkpoint, error) {
	options := &checkpointOptions{}
	for _, opt := range opts {
		opt(options)
	}

	params := url.Values{}
	params.Set("hash", hash)
	params.Set("full", fmt.Sprintf("%t", options.full))

	result := new(Checkpoint)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointCheckpointsByHash, params.Encode()), result)
}

// GetCheckpointByNumber retrieves a checkpoint by number. Use WithFullTransactions to fetch full transaction details.
func (client *Client) GetCheckpointByNumber(ctx context.Context, number uint64, opts ...CheckpointOption) (*Checkpoint, error) {
	options := &checkpointOptions{}
	for _, opt := range opts {
		opt(options)
	}

	params := url.Values{}
	params.Set("number", strconv.FormatUint(number, 10))
	params.Set("full", fmt.Sprintf("%t", options.full))

	result := new(Checkpoint)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointCheckpointsByNumber, params.Encode()), result)
}

// GetCheckpointReceiptsByNumber retrieves transaction receipts for a checkpoint by number.
func (client *Client) GetCheckpointReceiptsByNumber(ctx context.Context, number uint64) ([]TransactionReceiptResponse, error) {
	params := url.Values{}
	params.Set("number", strconv.FormatUint(number, 10))

	var result []TransactionReceiptResponse
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointCheckpointsReceiptsByNumber, params.Encode()), &result)
}
