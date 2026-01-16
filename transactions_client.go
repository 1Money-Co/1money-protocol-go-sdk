package onemoney

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

const (
	endpointTransactionsByHashV1        = "/v1/transactions/by_hash"
	endpointTransactionsReceiptByHashV1 = "/v1/transactions/receipt/by_hash"
	endpointTransactionsFinalizedV1     = "/v1/transactions/finalized/by_hash"
	endpointTransactionsEstimateFeeV1   = "/v1/transactions/estimate_fee"
	endpointTransactionsPaymentV1       = "/v1/transactions/payment"
)

// GetTransactionByHash retrieves a transaction by its hash.
func (client *Client) GetTransactionByHash(ctx context.Context, hash string) (*Transaction, error) {
	result := new(Transaction)
	params := url.Values{}
	params.Set("hash", hash)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTransactionsByHashV1, params.Encode()), result)
}

// GetTransactionReceipt retrieves a transaction receipt by its hash.
func (client *Client) GetTransactionReceipt(ctx context.Context, hash string) (*TransactionReceiptResponse, error) {
	result := new(TransactionReceiptResponse)
	params := url.Values{}
	params.Set("hash", hash)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTransactionsReceiptByHashV1, params.Encode()), result)
}

// GetFinalizedTransaction retrieves a finalized transaction by its hash.
func (client *Client) GetFinalizedTransaction(ctx context.Context, hash string) (*FinalizedTransactionResponse, error) {
	result := new(FinalizedTransactionResponse)
	params := url.Values{}
	params.Set("hash", hash)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTransactionsFinalizedV1, params.Encode()), result)
}

// GetEstimateFee retrieves the estimated fee for a transaction.
func (client *Client) GetEstimateFee(ctx context.Context, from, to, token common.Address, value string) (*EstimateFeeResponse, error) {
	result := new(EstimateFeeResponse)
	params := url.Values{}
	params.Set("from", from.Hex())
	params.Set("to", to.Hex())
	params.Set("token", token.Hex())
	params.Set("value", value)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTransactionsEstimateFeeV1, params.Encode()), result)
}

// SendPayment sends a payment transaction to the network.
func (client *Client) SendPayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	result := new(PaymentResponse)
	return result, client.PostMethod(ctx, endpointTransactionsPaymentV1, req, result)
}
