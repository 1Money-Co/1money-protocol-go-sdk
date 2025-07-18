package onemoney

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"net/url"
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
	TransactionType string `json:"transaction_type"`
	// Data holds the specific payload for the transaction, which varies based on TransactionType.
	// Common examples include TokenCreatePayload or TokenTransferPayload.
	// Using interface{} (or any) for flexibility as new transaction types can be added.
	// Consumers of this struct typically use a type switch or assertion on Data based on TransactionType.
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

func (client *Client) GetTransactionByHash(ctx context.Context, hash string) (*Transaction, error) {
	result := new(Transaction)
	endpoint := "/v1/transactions/by_hash"
	params := url.Values{}
	params.Set("hash", hash)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpoint, params.Encode()), result)
}

type TransactionReceiptResponse struct {
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

func (client *Client) GetTransactionReceipt(ctx context.Context, hash string) (*TransactionReceiptResponse, error) {
	result := new(TransactionReceiptResponse)
	endpoint := "/v1/transactions/receipt/by_hash"
	params := url.Values{}
	params.Set("hash", hash)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpoint, params.Encode()), result)
}

type EstimateFeeResponse struct {
	Fee string `json:"fee"`
}

func (client *Client) GetEstimateFee(ctx context.Context, from, token, value string) (*EstimateFeeResponse, error) {
	result := new(EstimateFeeResponse)
	endpoint := "/v1/transactions/estimate_fee"
	params := url.Values{}
	params.Set("from", from)
	params.Set("token", token)
	params.Set("value", value)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpoint, params.Encode()), result)
}

type PaymentPayload struct {
	RecentEpoch      uint64         `json:"recent_epoch"`
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Recipient        common.Address `json:"recipient"`
	Value            *big.Int       `json:"value"`
	Token            common.Address `json:"token"`
}

type PaymentRequest struct {
	PaymentPayload
	Signature Signature `json:"signature"`
}

type PaymentResponse struct {
	Hash string `json:"hash"`
}

func (client *Client) SendPayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	result := new(PaymentResponse)
	return result, client.PostMethod(ctx, "/v1/transactions/payment", req, result)
}
