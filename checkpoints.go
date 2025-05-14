package onemoney

import (
	"fmt"
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

func (api *Client) GetCheckpointNumber() (*CheckpointNumber, error) {
	result := new(CheckpointNumber)
	return result, api.GetMethod("/v1/checkpoints/number", result)
}

func (api *Client) GetCheckpointByHashFull(hash string) (*CheckpointDetailFull, error) {
	result := new(CheckpointDetailFull)
	return result, api.GetMethod(fmt.Sprintf("/v1/checkpoints/by_hash?hash=%s&full=%v", hash, true), result)
}

func (api *Client) GetCheckpointByHash(hash string) (*CheckpointDetail, error) {
	result := new(CheckpointDetail)
	return result, api.GetMethod(fmt.Sprintf("/v1/checkpoints/by_hash?hash=%s&full=%v", hash, false), result)
}

func (api *Client) GetCheckpointByNumberFull(number int) (*CheckpointDetailFull, error) {
	result := new(CheckpointDetailFull)
	return result, api.GetMethod(fmt.Sprintf("/v1/checkpoints/by_number?number=%d&full=%v", number, true), result)
}

func (api *Client) GetCheckpointByNumber(number int) (*CheckpointDetail, error) {
	result := new(CheckpointDetail)
	return result, api.GetMethod(fmt.Sprintf("/v1/checkpoints/by_number?number=%d&full=%v", number, false), result)
}
