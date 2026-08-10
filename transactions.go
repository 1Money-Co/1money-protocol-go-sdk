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

// SendPayment sends a pre-signed payment transaction to the network.
//
// Deprecated: use Transactions().Payment, which signs internally with a Signer
// and defaults to domain-separated v2. SendPayment posts a legacy v1 request.
func (client *Client) SendPayment(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	result := new(PaymentResponse)
	return result, client.PostMethod(ctx, endpointTransactionsPaymentV1, req, result)
}

// -----------------------------------------------------------------------------
// Domain-separated v2 transaction writes (namespace API)
// -----------------------------------------------------------------------------

// TransactionsAPI groups the domain-separated transaction submit methods.
// Signing, RLP encoding, memo canonicalization, endpoint selection, and hash
// verification are all handled internally; callers pass only a payload and a
// Signer. v2 is used by default; WithLegacyV1 on the client selects /v1.
type TransactionsAPI struct{ c *Client }

// Transactions returns the transaction submit namespace.
func (c *Client) Transactions() TransactionsAPI { return TransactionsAPI{c: c} }

// Payment signs and submits a payment transaction.
func (a TransactionsAPI) Payment(ctx context.Context, payload PaymentPayload, signer Signer, opts ...SubmitOption) (*PaymentResponse, error) {
	out := new(PaymentResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// BatchPayment signs and submits a batch payment. Batch payments are
// memo-bearing like every other canonical v2 operation: pass WithMemo to attach
// one, otherwise the canonical empty memo is signed.
func (a TransactionsAPI) BatchPayment(ctx context.Context, payload BatchPaymentPayload, signer Signer, opts ...SubmitOption) (*PaymentResponse, error) {
	out := new(PaymentResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}
