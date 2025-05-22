package onemoney

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type CheckpointNumber struct {
	Number int `json:"number"`
}

type TokenData struct {
	Decimals        string `json:"decimals"`
	MasterAuthority string `json:"master_authority"`
	Symbol          string `json:"symbol"`
}

type CheckpointDetailFull struct {
	ExtraData        string        `json:"extra_data"`
	Hash             string        `json:"hash"`
	Number           uint64        `json:"number"`
	ParentHash       string        `json:"parent_hash"`
	ReceiptsRoot     string        `json:"receipts_root"`
	StateRoot        string        `json:"state_root"`
	Timestamp        uint64        `json:"timestamp"`
	TransactionsRoot string        `json:"transactions_root"`
	Transactions     []Transaction `json:"transactions"`
	Size             int           `json:"size"`
}

type CheckpointDetail struct {
	ExtraData        string   `json:"extra_data"`
	Hash             string   `json:"hash"`
	Number           uint64   `json:"number"`
	ParentHash       string   `json:"parent_hash"`
	ReceiptsRoot     string   `json:"receipts_root"`
	StateRoot        string   `json:"state_root"`
	Timestamp        uint64   `json:"timestamp"`
	TransactionsRoot string   `json:"transactions_root"`
	Transactions     []string `json:"transactions"`
	Size             int      `json:"size"`
}

func (client *Client) GetCheckpointNumber(ctx context.Context) (*CheckpointNumber, error) {
	result := new(CheckpointNumber)
	return result, client.GetMethod(ctx, "/v1/checkpoints/number", result)
}

func (client *Client) GetCheckpointByHashFull(ctx context.Context, hash string) (*CheckpointDetailFull, error) {
	result := new(CheckpointDetailFull)
	params := url.Values{}
	params.Set("hash", hash)
	params.Set("full", "true")
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/checkpoints/by_hash?%s", params.Encode()), result)
}

func (client *Client) GetCheckpointByHash(ctx context.Context, hash string) (*CheckpointDetail, error) {
	result := new(CheckpointDetail)
	params := url.Values{}
	params.Set("hash", hash)
	params.Set("full", "false")
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/checkpoints/by_hash?%s", params.Encode()), result)
}

func (client *Client) GetCheckpointByNumberFull(ctx context.Context, number int) (*CheckpointDetailFull, error) {
	result := new(CheckpointDetailFull)
	params := url.Values{}
	params.Set("number", strconv.Itoa(number))
	params.Set("full", "true")
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/checkpoints/by_number?%s", params.Encode()), result)
}

func (client *Client) GetCheckpointByNumber(ctx context.Context, number int) (*CheckpointDetail, error) {
	result := new(CheckpointDetail)
	params := url.Values{}
	params.Set("number", strconv.Itoa(number))
	params.Set("full", "false")
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/checkpoints/by_number?%s", params.Encode()), result)
}
