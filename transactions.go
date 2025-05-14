package onemoney

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
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
	result := new(Transaction)
	return result, api.GetMethod(fmt.Sprintf("/v1/transactions/by_hash?hash=%s", hash), result)
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

func (api *Client) GetTransactionReceipt(hash string) (*TransactionReceiptResponse, error) {
	result := new(TransactionReceiptResponse)
	return result, api.GetMethod(fmt.Sprintf("/v1/transactions/receipt/by_hash?hash=%s", hash), result)
}

type EstimateFeeResponse struct {
	Fee string `json:"fee"`
}

func (api *Client) GetEstimateFee(from, token, value string) (*EstimateFeeResponse, error) {
	result := new(EstimateFeeResponse)
	return result, api.GetMethod(fmt.Sprintf(api.baseUrl+"/v1/transactions/estimate_fee?from=%s&token=%s&value=%s", from, token, value), result)
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
	result := new(PaymentResponse)
	return result, api.PostMethod("/v1/transactions/payment", req, result)
}
