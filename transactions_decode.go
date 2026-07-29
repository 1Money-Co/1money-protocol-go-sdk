package onemoney

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
)

// This file owns transaction-response decoding: the TransactionType -> payload
// constructor registry, Transaction.UnmarshalJSON discrimination, the typed As*
// accessors, and the isTransactionPayload markers that close the payload union.
// The payload struct definitions themselves live in transactions_payloads.go.

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

var defaultTransactionPayloadConstructors = map[TransactionType]func() TransactionPayload{
	TransactionTypeTokenCreate:           func() TransactionPayload { return &TokenCreateData{} },
	TransactionTypeTokenTransfer:         func() TransactionPayload { return &TokenTransferData{} },
	TransactionTypeBatchPayment:          func() TransactionPayload { return &BatchPaymentData{} },
	TransactionTypeTokenClawback:         func() TransactionPayload { return &TokenClawbackData{} },
	TransactionTypeTokenGrantAuthority:   func() TransactionPayload { return &TokenGrantAuthorityData{} },
	TransactionTypeTokenRevokeAuthority:  func() TransactionPayload { return &TokenRevokeAuthorityData{} },
	TransactionTypeTokenBlacklistAccount: func() TransactionPayload { return &TokenBlacklistAccountData{} },
	TransactionTypeTokenWhitelistAccount: func() TransactionPayload { return &TokenWhitelistAccountData{} },
	TransactionTypeTokenMint:             func() TransactionPayload { return &TokenMintData{} },
	TransactionTypeTokenBridgeAndMint:    func() TransactionPayload { return &TokenBridgeAndMintData{} },
	TransactionTypeTokenBurn:             func() TransactionPayload { return &TokenBurnData{} },
	TransactionTypeTokenBurnAndBridge:    func() TransactionPayload { return &TokenBurnAndBridgeData{} },
	TransactionTypeTokenCloseAccount:     func() TransactionPayload { return &TokenCloseAccountData{} },
	TransactionTypeTokenPause:            func() TransactionPayload { return &TokenPauseData{} },
	TransactionTypeTokenUnpause:          func() TransactionPayload { return &TokenUnpauseData{} },
	TransactionTypeTokenUpdateMetadata:   func() TransactionPayload { return &TokenUpdateMetadataData{} },
	TransactionTypeCreateMultiSig:        func() TransactionPayload { return &CreateMultiSigData{} },
	TransactionTypeEmpty:                 func() TransactionPayload { return &EmptyData{} },
	TransactionTypeRaw:                   func() TransactionPayload { return &RawTransactionData{} },
}

func init() {
	for tt, ctor := range defaultTransactionPayloadConstructors {
		RegisterTransactionPayload(tt, ctor)
	}
}

// UnknownTransactionPayload captures payloads for transaction types the SDK
// does not yet recognize.
type UnknownTransactionPayload map[string]interface{}

func (UnknownTransactionPayload) isTransactionPayload() {}

// UnmarshalJSON implements custom JSON unmarshaling for Transaction.
// It parses the Data field into the correct type based on TransactionType, and
// the polymorphic authorization (top-level signature_type + signature) into
// either Signature (Single) or MultiSignature (Multi).
func (t *Transaction) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a temporary struct to get TransactionType, the raw
	// data payload, and the raw signature content.
	type Alias Transaction
	aux := &struct {
		RawData      json.RawMessage `json:"data"`
		RawSignature json.RawMessage `json:"signature"`
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
	} else {
		// For unknown types, keep the payload as raw JSON.
		var rawData UnknownTransactionPayload
		if len(aux.RawData) > 0 {
			if err := json.Unmarshal(aux.RawData, &rawData); err != nil {
				return fmt.Errorf("failed to unmarshal unknown transaction data: %w", err)
			}
		}
		t.Data = rawData
	}

	// Parse the authorization based on signature_type. "Multi" carries an
	// account + per-signer entries; anything else (including "Single") carries a
	// single r/s/v signature.
	if len(aux.RawSignature) > 0 && !bytes.Equal(aux.RawSignature, []byte("null")) {
		switch t.SignatureType {
		case "Multi":
			multi := new(MultiSigSignature)
			if err := json.Unmarshal(aux.RawSignature, multi); err != nil {
				return fmt.Errorf("failed to unmarshal multisig signature: %w", err)
			}
			t.MultiSignature = multi
		default:
			sig := new(Signature)
			if err := json.Unmarshal(aux.RawSignature, sig); err != nil {
				return fmt.Errorf("failed to unmarshal signature: %w", err)
			}
			t.Signature = sig
		}
	}

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

func (t *Transaction) AsBatchPaymentData() (*BatchPaymentData, bool) {
	return asPayload[*BatchPaymentData](t, TransactionTypeBatchPayment)
}

func (t *Transaction) AsTokenClawbackData() (*TokenClawbackData, bool) {
	return asPayload[*TokenClawbackData](t, TransactionTypeTokenClawback)
}

func (t *Transaction) AsCreateMultiSigData() (*CreateMultiSigData, bool) {
	return asPayload[*CreateMultiSigData](t, TransactionTypeCreateMultiSig)
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

// isTransactionPayload markers keep the payload interface closed to known types.
func (*TokenCreateData) isTransactionPayload()           {}
func (*EmptyData) isTransactionPayload()                 {}
func (*TokenTransferData) isTransactionPayload()         {}
func (*BatchPaymentData) isTransactionPayload()          {}
func (*TokenClawbackData) isTransactionPayload()         {}
func (*CreateMultiSigData) isTransactionPayload()        {}
func (*TokenGrantAuthorityData) isTransactionPayload()   {}
func (*TokenRevokeAuthorityData) isTransactionPayload()  {}
func (*TokenBlacklistAccountData) isTransactionPayload() {}
func (*TokenWhitelistAccountData) isTransactionPayload() {}
func (*TokenMintData) isTransactionPayload()             {}
func (*TokenBridgeAndMintData) isTransactionPayload()    {}
func (*TokenBurnData) isTransactionPayload()             {}
func (*TokenBurnAndBridgeData) isTransactionPayload()    {}
func (*TokenCloseAccountData) isTransactionPayload()     {}
func (*TokenPauseData) isTransactionPayload()            {}
func (*TokenUnpauseData) isTransactionPayload()          {}
func (*TokenUpdateMetadataData) isTransactionPayload()   {}
func (*RawTransactionData) isTransactionPayload()        {}
