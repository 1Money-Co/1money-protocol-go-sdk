package onemoney

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type B256 string
type Bytes []byte

type TransactionType string

const (
	TransactionTypeTokenCreate           TransactionType = "TokenCreate"
	TransactionTypeTokenTransfer         TransactionType = "TokenTransfer"
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
	TransactionTypeEmpty                 TransactionType = "Empty"
	TransactionTypeRaw                   TransactionType = "Raw"
)

// TransactionPayload marks a struct as a valid transaction payload returned by the API.
type TransactionPayload interface {
	// isTransactionPayload is a no-op marker used to keep the interface closed to known types.
	isTransactionPayload()
}

// RegisterTransactionPayload adds or overrides the constructor used to instantiate
// the payload for a specific TransactionType.
func RegisterTransactionPayload(tt TransactionType, ctor func() TransactionPayload) {
	transactionPayloadRegistryMu.Lock()
	defer transactionPayloadRegistryMu.Unlock()
	transactionPayloadRegistry[tt] = ctor
}

var (
	transactionPayloadRegistryMu sync.RWMutex
	transactionPayloadRegistry   = make(map[TransactionType]func() TransactionPayload)
)

func newTransactionPayload(tt TransactionType) (TransactionPayload, bool) {
	transactionPayloadRegistryMu.RLock()
	defer transactionPayloadRegistryMu.RUnlock()
	ctor, ok := transactionPayloadRegistry[tt]
	if !ok {
		return nil, false
	}
	return ctor(), true
}

// UnknownTransactionPayload captures payloads for transaction types the SDK
// does not yet recognize.
type UnknownTransactionPayload map[string]interface{}

func (UnknownTransactionPayload) isTransactionPayload() {}

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

// UnmarshalJSON implements custom JSON unmarshaling for Transaction.
// It automatically parses the Data field into the correct type based on TransactionType.
func (t *Transaction) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a temporary struct to get TransactionType.
	type Alias Transaction
	aux := &struct {
		RawData json.RawMessage `json:"data"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse Data based on TransactionType.
	if payload, ok := newTransactionPayload(t.TransactionType); ok {
		if len(aux.RawData) > 0 {
			if err := json.Unmarshal(aux.RawData, payload); err != nil {
				return fmt.Errorf("failed to unmarshal %s data: %w", t.TransactionType, err)
			}
		}
		t.Data = payload
		return nil
	}

	// For unknown types, keep as raw JSON.
	var rawData UnknownTransactionPayload
	if len(aux.RawData) > 0 {
		if err := json.Unmarshal(aux.RawData, &rawData); err != nil {
			return fmt.Errorf("failed to unmarshal unknown transaction data: %w", err)
		}
	}
	t.Data = rawData

	return nil
}

// Type-safe helper methods to access transaction data.

func asPayload[T TransactionPayload](t *Transaction, expected TransactionType) (T, bool) {
	var zero T
	if t.TransactionType != expected {
		return zero, false
	}
	payload, ok := t.Data.(T)
	if !ok {
		return zero, false
	}
	return payload, true
}

func (t *Transaction) AsTokenCreateData() (*TokenCreateData, bool) {
	return asPayload[*TokenCreateData](t, TransactionTypeTokenCreate)
}

func (t *Transaction) AsTokenTransferData() (*TokenTransferData, bool) {
	return asPayload[*TokenTransferData](t, TransactionTypeTokenTransfer)
}

func (t *Transaction) AsTokenGrantAuthorityData() (*TokenGrantAuthorityData, bool) {
	return asPayload[*TokenGrantAuthorityData](t, TransactionTypeTokenGrantAuthority)
}

func (t *Transaction) AsTokenRevokeAuthorityData() (*TokenRevokeAuthorityData, bool) {
	return asPayload[*TokenRevokeAuthorityData](t, TransactionTypeTokenRevokeAuthority)
}

func (t *Transaction) AsTokenBlacklistAccountData() (*TokenBlacklistAccountData, bool) {
	return asPayload[*TokenBlacklistAccountData](t, TransactionTypeTokenBlacklistAccount)
}

func (t *Transaction) AsTokenWhitelistAccountData() (*TokenWhitelistAccountData, bool) {
	return asPayload[*TokenWhitelistAccountData](t, TransactionTypeTokenWhitelistAccount)
}

func (t *Transaction) AsTokenMintData() (*TokenMintData, bool) {
	return asPayload[*TokenMintData](t, TransactionTypeTokenMint)
}

func (t *Transaction) AsTokenBridgeAndMintData() (*TokenBridgeAndMintData, bool) {
	return asPayload[*TokenBridgeAndMintData](t, TransactionTypeTokenBridgeAndMint)
}

func (t *Transaction) AsTokenBurnData() (*TokenBurnData, bool) {
	return asPayload[*TokenBurnData](t, TransactionTypeTokenBurn)
}

func (t *Transaction) AsTokenBurnAndBridgeData() (*TokenBurnAndBridgeData, bool) {
	return asPayload[*TokenBurnAndBridgeData](t, TransactionTypeTokenBurnAndBridge)
}

func (t *Transaction) AsTokenCloseAccountData() (*TokenCloseAccountData, bool) {
	return asPayload[*TokenCloseAccountData](t, TransactionTypeTokenCloseAccount)
}

func (t *Transaction) AsTokenPauseData() (*TokenPauseData, bool) {
	return asPayload[*TokenPauseData](t, TransactionTypeTokenPause)
}

func (t *Transaction) AsTokenUnpauseData() (*TokenUnpauseData, bool) {
	return asPayload[*TokenUnpauseData](t, TransactionTypeTokenUnpause)
}

func (t *Transaction) AsTokenUpdateMetadataData() (*TokenUpdateMetadataData, bool) {
	return asPayload[*TokenUpdateMetadataData](t, TransactionTypeTokenUpdateMetadata)
}

func (t *Transaction) AsEmptyData() (*EmptyData, bool) {
	return asPayload[*EmptyData](t, TransactionTypeEmpty)
}

func (t *Transaction) AsRawTransactionData() (*RawTransactionData, bool) {
	return asPayload[*RawTransactionData](t, TransactionTypeRaw)
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

type PaymentRequest struct {
	PaymentPayload
	Signature Signature `json:"signature"`
}

type PaymentResponse struct {
	Hash string `json:"hash"`
}

// Hash returns the transaction hash for the request (payload + signature).
func (r PaymentRequest) Hash() (common.Hash, error) {
	return Hash(r.PaymentPayload, r.Signature)
}
