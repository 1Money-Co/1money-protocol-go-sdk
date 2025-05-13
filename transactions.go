package onemoney

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"math/big"
	"net/http"
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

func (api *Client) GetTransactionByHash(hash string) (*Transaction, error) {
	gin.SetMode(gin.ReleaseMode)
	url := fmt.Sprintf(api.baseUrl+"/v1/transactions/by_hash?hash=%s", hash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	var result Transaction
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
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

func (api *Client) GetTransactionReceipt(hash string) (*TransactionReceipt, error) {
	gin.SetMode(gin.ReleaseMode)
	url := fmt.Sprintf(api.baseUrl+"/v1/transactions/receipt/by_hash?hash=%s", hash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}
	var result TransactionReceipt
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type EstimateFee struct {
	Fee string `json:"fee"`
}

func (api *Client) GetEstimateFee(from, token, value string) (*EstimateFee, error) {
	gin.SetMode(gin.ReleaseMode)
	url := fmt.Sprintf(api.baseUrl+"/v1/transactions/estimate_fee?from=%s&token=%s&value=%s", from, token, value)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get estimate fee: %w", err)
	}
	var result EstimateFee
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
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

func (api *Client) SendPayment(req *PaymentRequest) (*PaymentResponse, error) {
	gin.SetMode(gin.ReleaseMode)
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq, err := http.NewRequest("POST", api.baseUrl+"/v1/transactions/payment", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send payment: %w", err)
	}
	var result PaymentResponse
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
