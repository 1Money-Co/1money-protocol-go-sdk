package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"

	"go-1money/config"
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

func GetTransactionByHash(hash string) (*Transaction, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf(config.BaseAPIURL+"/v1/transactions/by_hash?hash=%s", hash)
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

	url := fmt.Sprintf(config.BaseAPIURL+"/v1/transactions/receipt/by_hash?hash=%s", hash)
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

type EstimateFee struct {
	Fee string `json:"fee"`
}

func GetEstimateFee(from, token, value string) (*EstimateFee, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf(config.BaseAPIURL+"/v1/transactions/estimate_fee?from=%s&token=%s&value=%s", from, token, value)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get estimate fee: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result EstimateFee
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

type PaymentPayload struct {
	ChainID   uint64         `json:"chain_id"`
	Nonce     uint64         `json:"nonce"`
	Recipient common.Address `json:"recipient"`
	Value     *big.Int       `json:"value"`
	Token     common.Address `json:"token"`
}

type PaymentRequest struct {
	PaymentPayload
	Signature Signature `json:"signature"`
}

type PaymentResponse struct {
	Hash string `json:"hash"`
}

func SendPayment(req *PaymentRequest) (*PaymentResponse, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := config.BaseAPIURL + "/v1/transactions/payment"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send payment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
