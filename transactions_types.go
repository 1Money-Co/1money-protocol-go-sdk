package onemoney

import (
	"encoding/json"
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
	// CheckpointHash, CheckpointNumber and TransactionIndex are nil until the
	// transaction is included in a checkpoint (the node returns null before then).
	CheckpointHash   *string         `json:"checkpoint_hash"`
	CheckpointNumber *uint64         `json:"checkpoint_number"`
	TransactionIndex *uint64         `json:"transaction_index"`
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
	Data TransactionPayload `json:"-"`
	// SignatureType discriminates the authorization: "Single" or "Multi". It
	// selects which of Signature / MultiSignature is populated.
	SignatureType string `json:"signature_type,omitempty"`
	// Signature is set when SignatureType == "Single" (the common single-signer
	// case). It is nil for multisig transactions.
	Signature *Signature `json:"-"`
	// MultiSignature is set when SignatureType == "Multi" (multisig account). It
	// is nil for single-signer transactions.
	MultiSignature *MultiSigSignature `json:"-"`
	// Memo is the optional signed memo (domain-separated v2). nil when absent.
	Memo *Memo `json:"memo,omitempty"`
	// SignatureScheme reports how the transaction was signed (legacy_native,
	// domain_separated, ethereum, eip712). Empty when the node omits it.
	SignatureScheme SignatureScheme `json:"signature_scheme,omitempty"`
}

// SignatureScheme identifies how a transaction was signed, as reported by the
// node. The wire values are lowercase snake_case.
type SignatureScheme string

const (
	// SignatureSchemeLegacyNative is the legacy v1 native signing scheme.
	SignatureSchemeLegacyNative SignatureScheme = "legacy_native"
	// SignatureSchemeDomainSeparated is the domain-separated v2 signing scheme (#1038).
	SignatureSchemeDomainSeparated SignatureScheme = "domain_separated"
	// SignatureSchemeEthereum is an Ethereum-compatible signature.
	SignatureSchemeEthereum SignatureScheme = "ethereum"
	// SignatureSchemeEip712 is an EIP-712 typed-data signature.
	SignatureSchemeEip712 SignatureScheme = "eip712"
)

// MultiSigSignature is the authorization carried by a multisig transaction
// (Transaction.SignatureType == "Multi").
type MultiSigSignature struct {
	Account    common.Address           `json:"account"`
	Signatures []MultiSigSignatureEntry `json:"signatures"`
}

// MultiSigSignatureEntry is one signer's contribution to a multisig signature.
type MultiSigSignatureEntry struct {
	SignerPubkey string    `json:"signer_pubkey"` // 0x-hex, 33-byte compressed pubkey
	Signature    Signature `json:"signature"`
}

// -----------------------------------------------------------------------------
// Transaction API DTOs
// -----------------------------------------------------------------------------

type TransactionReceiptResponse struct {
	Success         bool   `json:"success"`
	TransactionHash string `json:"transaction_hash"`
	// TransactionIndex, CheckpointHash and CheckpointNumber are nil until the
	// transaction is finalized in a checkpoint (the node returns null before then).
	TransactionIndex *uint64         `json:"transaction_index"`
	CheckpointHash   *string         `json:"checkpoint_hash"`
	CheckpointNumber *uint64         `json:"checkpoint_number"`
	From             common.Address  `json:"from"`
	FeeUsed          string          `json:"fee_used"`
	Recipient        *common.Address `json:"recipient"`
	TokenAddress     *common.Address `json:"token_address"` // Pointer to handle null values
	// SuccessInfo carries execution detail for a successful transaction; nil when absent.
	SuccessInfo *SuccessInfo `json:"success_info,omitempty"`
	// BatchInfo carries batch-payment detail for a batch receipt; nil for non-batch.
	BatchInfo *BatchReceiptInfo `json:"batch_info,omitempty"`
	// ExecutionEvents lists per-operation events (e.g. batch payment); empty when absent.
	ExecutionEvents []ExecutionEvent `json:"execution_events,omitempty"`
}

// SuccessInfo carries the execution detail of a successful transaction.
type SuccessInfo struct {
	Sender     common.Address `json:"sender"`
	Receiver   common.Address `json:"receiver"`
	IsPrivate  bool           `json:"is_private"`
	Message    string         `json:"message"`
	BridgeInfo *BridgeInfo    `json:"bridge_info"`
}

// BridgeInfo carries cross-chain bridge detail attached to a successful bridge transaction.
type BridgeInfo struct {
	BbNonce            uint64 `json:"bbnonce"`
	DestinationChainID uint64 `json:"destination_chain_id"`
	DestinationAddress string `json:"destination_address"`
	BridgeParam        string `json:"bridge_param"` // 0x-hex
}

// BatchReceiptInfo carries batch-payment detail for a batch transaction receipt.
type BatchReceiptInfo struct {
	BatchID         *string           `json:"batch_id"`
	OperationsHash  *common.Hash      `json:"operations_hash"`
	OperationsCount uint64            `json:"operations_count"`
	TotalAmount     string            `json:"total_amount"`
	Failure         *BatchFailureInfo `json:"failure"`
}

// BatchFailureInfo identifies the operation that failed within a batch payment.
type BatchFailureInfo struct {
	FailedOperationIndex uint64 `json:"failed_operation_index"`
	Reason               string `json:"reason"`
}

// ExecutionEvent is one execution event emitted during transaction processing.
// It is internally tagged by EventType (BatchStarted, PaymentExecuted,
// BatchCompleted); only the fields relevant to EventType are populated.
type ExecutionEvent struct {
	EventType       string          `json:"event_type"`
	BatchID         *string         `json:"batch_id,omitempty"`
	OperationsCount *uint64         `json:"operations_count,omitempty"`
	TotalAmount     *string         `json:"total_amount,omitempty"`
	OperationsHash  *common.Hash    `json:"operations_hash,omitempty"`
	OperationIndex  *uint64         `json:"operation_index,omitempty"`
	Recipient       *common.Address `json:"recipient,omitempty"`
	Amount          *string         `json:"amount,omitempty"`
}

type FinalizedTransactionResponse struct {
	TransactionReceiptResponse
	Epoch uint64 `json:"epoch"`
	// CounterSignature is the BLS aggregate of validator counter-signatures.
	CounterSignature BlsAggregateSignature `json:"counter_signature"`
	// Fee is the decimal fee charged; nil when the node omits it (older payloads).
	Fee *string `json:"fee"`
	// FeeBound reports whether the fee was bound at signing time (Security #1151).
	FeeBound bool `json:"fee_bound"`
}

// BlsAggregateSignature is the aggregated validator counter-signature attached
// to a finalized transaction.
type BlsAggregateSignature struct {
	SignerBitmask       string   `json:"signer_bitmask"`
	Signature           string   `json:"signature"`
	ValidatorPublicKeys []string `json:"validator_public_keys"`
}

type EstimateFeeResponse struct {
	Fee string `json:"fee"`
	// Plan is the pricing plan applied to the estimate; nil when absent.
	Plan *string `json:"plan,omitempty"`
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
	CreatedAt      uint64             `json:"created_at"`
	OperationsHash *common.Hash       `json:"operations_hash,omitempty"`
	BatchID        *string            `json:"batch_id,omitempty"`
}

type PaymentResponse struct {
	Hash string `json:"hash"`
}

// TxHash reports the submitted transaction hash for hash-verification.
func (r *PaymentResponse) TxHash() string { return r.Hash }

// BatchPaymentFeeEstimateRequest is the unsigned input to the batch-payment
// fee-estimate endpoint. It carries no nonce, timestamp, memo, authorization, or
// operations hash: the node cannot validate those from an unsigned request, and
// the returned quote is non-binding.
type BatchPaymentFeeEstimateRequest struct {
	From       common.Address     `json:"from"`
	Token      common.Address     `json:"token"`
	Operations []PaymentOperation `json:"operations"`
}

// MarshalJSON renders the request with the same operation encoder the v2 submit
// body uses, so amounts are quoted decimal strings rather than the bare JSON
// numbers a default *big.Int marshal would emit. Client.GetBatchPaymentEstimateFee
// and a caller's direct json.Marshal therefore produce identical bodies.
func (r BatchPaymentFeeEstimateRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"from":       r.From,
		"token":      r.Token,
		"operations": batchOperationsWireList(r.Operations),
	})
}
