package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-1money/config"
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

func GetCheckpointNumber() (*CheckpointNumber, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	req, err := http.NewRequest("GET", config.BaseAPIURL+"/v1/checkpoints/number", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint number: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result CheckpointNumber
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func GetCheckpointByNumber(number int, full bool) (*CheckpointDetail, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf(config.BaseAPIURL+"/v1/checkpoints/by_number?number=%d&full=%v", number, full)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint detail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result CheckpointDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
