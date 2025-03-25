package checkpoints

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CheckpointNumber struct {
	Number int `json:"number"`
}

type Signature struct {
	R string `json:"r"`
	S string `json:"s"`
	V string `json:"v"`
}

type TokenData struct {
	Decimals        string `json:"decimals"`
	MasterAuthority string `json:"master_authority"`
	Symbol          string `json:"symbol"`
}

type Transaction struct {
	Data             *TokenData `json:"data,omitempty"`
	TransactionType  string     `json:"transaction_type,omitempty"`
	ChainID          string     `json:"chain_id,omitempty"`
	CheckpointHash   string     `json:"checkpoint_hash,omitempty"`
	CheckpointNumber string     `json:"checkpoint_number,omitempty"`
	Fee              string     `json:"fee,omitempty"`
	From             string     `json:"from,omitempty"`
	Hash             string     `json:"hash,omitempty"`
	Nonce            string     `json:"nonce,omitempty"`
	Signature        *Signature `json:"signature,omitempty"`
	TransactionIndex string     `json:"transaction_index,omitempty"`
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

	req, err := http.NewRequest("GET", "https://api.testnet.1money.network/v1/checkpoints/number", nil)
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

	url := fmt.Sprintf("https://api.testnet.1money.network/v1/checkpoints/by_number?number=%d&full=%v", number, full)
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
