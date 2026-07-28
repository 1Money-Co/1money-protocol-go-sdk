package onemoney

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type TransactionType string

const (
	TransactionTypeTokenCreate           TransactionType = "TokenCreate"
	TransactionTypeTokenTransfer         TransactionType = "TokenTransfer"
	TransactionTypeBatchPayment          TransactionType = "BatchPayment"
	TransactionTypeTokenClawback         TransactionType = "TokenClawback"
	TransactionTypeTokenGrantAuthority   TransactionType = "TokenGrantAuthority"
	TransactionTypeTokenRevokeAuthority  TransactionType = "TokenRevokeAuthority"
	TransactionTypeTokenBlacklistAccount TransactionType = "TokenBlacklistAccount"
	TransactionTypeTokenWhitelistAccount TransactionType = "TokenWhitelistAccount"
	TransactionTypeTokenMint             TransactionType = "TokenMint"
	TransactionTypeTokenBridgeAndMint    TransactionType = "TokenBridgeAndMint"
	TransactionTypeTokenBurn             TransactionType = "TokenBurn"
	TransactionTypeTokenBurnAndBridge    TransactionType = "TokenBurnAndBridge"
	TransactionTypeTokenCloseAccount     TransactionType = "TokenCloseAccount"
	TransactionTypeTokenPause            TransactionType = "TokenPause"
	TransactionTypeTokenUnpause          TransactionType = "TokenUnpause"
	TransactionTypeTokenUpdateMetadata   TransactionType = "TokenUpdateMetadata"
	TransactionTypeCreateMultiSig        TransactionType = "CreateMultiSig"
	TransactionTypeEmpty                 TransactionType = "Empty"
	TransactionTypeRaw                   TransactionType = "Raw"
)

// TransactionPayload marks a struct as a valid transaction payload returned by the API.
type TransactionPayload interface {
	// isTransactionPayload is a no-op marker used to keep the interface closed to known types.
	isTransactionPayload()
}

// -----------------------------------------------------------------------------
// Transaction Core Types
// -----------------------------------------------------------------------------

type Transaction struct {
	CheckpointHash   string          `json:"checkpoint_hash"`
	CheckpointNumber uint64          `json:"checkpoint_number"`
	TransactionIndex int             `json:"transaction_index"`
	Hash             string          `json:"hash"`
	From             common.Address  `json:"from"`
	ChainID          uint64          `json:"chain_id"`
	Nonce            uint64          `json:"nonce"`
	TransactionType  TransactionType `json:"transaction_type"`
	// Data holds the specific payload for the transaction, which varies based on TransactionType.
	//
	// The SDK automatically unmarshals Data into the appropriate type based on TransactionType.
	// Use type assertion or the helper methods (AsTokenCreateData, AsTokenMintData, etc.) to
	// access the typed data.
	Data      TransactionPayload `json:"-"`
	Signature *Signature         `json:"signature"`
}

// -----------------------------------------------------------------------------
// Transaction API DTOs
// -----------------------------------------------------------------------------

type TransactionReceiptResponse struct {
	Success          bool           `json:"success"`
	TransactionHash  string         `json:"transaction_hash"`
	TransactionIndex int            `json:"transaction_index"`
	CheckpointHash   string         `json:"checkpoint_hash"`
	CheckpointNumber uint64         `json:"checkpoint_number"`
	From             common.Address `json:"from"`
	FeeUsed          string         `json:"fee_used"`
	// Deprecated: To is deprecated, use `Recipient` instead.
	To           *common.Address `json:"to"`
	Recipient    *common.Address `json:"recipient"`
	TokenAddress *common.Address `json:"token_address"` // Pointer to handle null values
}

type FinalizedTransactionResponse struct {
	TransactionReceiptResponse
	Epoch             uint64      `json:"epoch"`
	CounterSignatures []Signature `json:"counter_signatures"`
}

type EstimateFeeResponse struct {
	Fee string `json:"fee"`
}

type PaymentPayload struct {
	ChainID   uint64         `json:"chain_id"`
	Nonce     uint64         `json:"nonce"`
	Recipient common.Address `json:"recipient"`
	Value     *big.Int       `json:"value"`
	Token     common.Address `json:"token"`
}

// PaymentOperation is one recipient/amount pair inside a batch payment.
type PaymentOperation struct {
	Recipient common.Address `json:"recipient"`
	Amount    *big.Int       `json:"amount"`
}

// BatchPaymentPayload pays many recipients of one token in a single
// transaction. operations_hash and batch_id are optional trailing fields.
type BatchPaymentPayload struct {
	ChainID        uint64             `json:"chain_id"`
	Nonce          uint64             `json:"nonce"`
	Token          common.Address     `json:"token"`
	Operations     []PaymentOperation `json:"operations"`
	MaxFee         *big.Int           `json:"max_fee"`
	CreatedAt      uint64             `json:"created_at"`
	OperationsHash *common.Hash       `json:"operations_hash,omitempty"`
	BatchID        *string            `json:"batch_id,omitempty"`
}

type PaymentResponse struct {
	Hash string `json:"hash"`
}

// TxHash reports the submitted transaction hash for hash-verification.
func (r *PaymentResponse) TxHash() string { return r.Hash }
