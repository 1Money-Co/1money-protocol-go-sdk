package onemoney

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

type CheckpointNumber struct {
	Number int `json:"number"`
}

type TokenData struct {
	Decimals        string `json:"decimals"`
	MasterAuthority string `json:"master_authority"`
	Symbol          string `json:"symbol"`
}

type CheckpointDetail struct {
	ExtraData        string        `json:"extra_data"`
	Hash             string        `json:"hash"`
	Number           string        `json:"number"`
	ParentHash       string        `json:"parent_hash"`
	ReceiptsRoot     string        `json:"receipts_root"`
	StateRoot        string        `json:"state_root"`
	Timestamp        string        `json:"timestamp"`
	TransactionsRoot string        `json:"transactions_root"`
	Size             int           `json:"size"`
	Transactions     []Transaction `json:"transactions"`
}

func (api *Client) GetCheckpointNumber() (*CheckpointNumber, error) {
	gin.SetMode(gin.ReleaseMode)
	req, err := http.NewRequest("GET", api.baseUrl+"/v1/checkpoints/number", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint number: %w", err)
	}
	var result CheckpointNumber
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *Client) GetCheckpointByNumber(number int, full bool) (*CheckpointDetail, error) {
	gin.SetMode(gin.ReleaseMode)
	url := fmt.Sprintf(api.baseUrl+"/v1/checkpoints/by_number?number=%d&full=%v", number, full)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint detail: %w", err)
	}
	var result CheckpointDetail
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
