package transactions

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Address string
type B256 string
type Bytes []byte

// TokenCreatePayload represents token creation data
type TokenCreatePayload struct {
	Symbol          string  `json:"symbol"`
	Decimals        uint8   `json:"decimals"`
	MasterAuthority Address `json:"master_authority"`
}

// TokenTransferPayload represents token transfer data
type TokenTransferPayload struct {
	Value string   `json:"value"`
	To    Address  `json:"to"`
	Token *Address `json:"token"`
}

// TokenMintPayload represents token minting data
type TokenMintPayload struct {
	Value   string  `json:"value"`
	Address Address `json:"address"`
	Token   Address `json:"token"`
}

type Signature struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

type Transaction struct {
	TransactionType  string      `json:"transaction_type"`
	Data             interface{} `json:"data"`
	ChainID          int         `json:"chain_id"`
	CheckpointHash   string      `json:"checkpoint_hash"`
	CheckpointNumber int         `json:"checkpoint_number"`
	Fee              int         `json:"fee"`
	From             string      `json:"from"`
	Hash             string      `json:"hash"`
	Nonce            int         `json:"nonce"`
	Signature        *Signature  `json:"signature"`
	TransactionIndex int         `json:"transaction_index"`
}

// UnmarshalJSON implements custom JSON unmarshaling
func (t *Transaction) UnmarshalJSON(data []byte) error {
	type TempTransaction struct {
		TransactionType  string          `json:"transaction_type"`
		Data             json.RawMessage `json:"data"`
		ChainID          int             `json:"chain_id"`
		CheckpointHash   string          `json:"checkpoint_hash"`
		CheckpointNumber int             `json:"checkpoint_number"`
		Fee              int             `json:"fee"`
		From             string          `json:"from"`
		Hash             string          `json:"hash"`
		Nonce            int             `json:"nonce"`
		Signature        *Signature      `json:"signature"`
		TransactionIndex int             `json:"transaction_index"`
	}

	var temp TempTransaction
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	t.TransactionType = temp.TransactionType
	t.ChainID = temp.ChainID
	t.CheckpointHash = temp.CheckpointHash
	t.CheckpointNumber = temp.CheckpointNumber
	t.Fee = temp.Fee
	t.From = temp.From
	t.Hash = temp.Hash
	t.Nonce = temp.Nonce
	t.Signature = temp.Signature
	t.TransactionIndex = temp.TransactionIndex

	switch temp.TransactionType {
	case "TokenCreate":
		var payload TokenCreatePayload
		if err := json.Unmarshal(temp.Data, &payload); err != nil {
			return err
		}
		t.Data = &payload
	case "TokenTransfer":
		var payload TokenTransferPayload
		if err := json.Unmarshal(temp.Data, &payload); err != nil {
			return err
		}
		t.Data = &payload
	case "TokenMint":
		var payload TokenMintPayload
		if err := json.Unmarshal(temp.Data, &payload); err != nil {
			return err
		}
		t.Data = &payload
	//TODO more structures here
	default:
		t.Data = temp.Data
	}

	return nil
}

func GetTransactionByHash(hash string) (*Transaction, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf("https://api.testnet.1money.network/v1/transactions/by_hash?hash=%s", hash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result Transaction
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

type TransactionReceipt struct {
	CheckpointHash   string `json:"checkpoint_hash"`
	CheckpointNumber int    `json:"checkpoint_number"`
	FeeUsed          int    `json:"fee_used"`
	From             string `json:"from"`
	Success          bool   `json:"success"`
	To               string `json:"to"`
	TokenAddress     string `json:"token_address"`
	TransactionHash  string `json:"transaction_hash"`
	TransactionIndex int    `json:"transaction_index"`
}

func GetTransactionReceipt(hash string) (*TransactionReceipt, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf("https://api.testnet.1money.network/v1/transactions/receipt/by_hash?hash=%s", hash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result TransactionReceipt
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
