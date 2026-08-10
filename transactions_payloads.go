package onemoney

import "github.com/ethereum/go-ethereum/common"

type TokenCreateData struct {
	Decimals        uint8          `json:"decimals"`
	IsPrivate       bool           `json:"is_private"`
	ClawbackEnabled bool           `json:"clawback_enabled"`
	MasterAuthority common.Address `json:"master_authority"`
	Name            string         `json:"name"`
	Symbol          string         `json:"symbol"`
}

type EmptyData struct{}

type TokenTransferData struct {
	Recipient common.Address `json:"recipient"`
	// Token is nil for a native-value transfer (the node returns null).
	Token *common.Address `json:"token"`
	Value string          `json:"value"`
}

// BatchPaymentOperationData is one recipient/amount pair inside a decoded
// "BatchPayment" transaction; Amount is a U256 decimal string.
type BatchPaymentOperationData struct {
	Recipient common.Address `json:"recipient"`
	Amount    string         `json:"amount"`
}

// BatchPaymentData is the decoded payload of a "BatchPayment" transaction — a
// single-token payment to many recipients. Token, OperationsHash, and BatchID
// mirror the node's optional fields (nil when absent), so a null never fails the
// whole transaction decode.
type BatchPaymentData struct {
	Token          *common.Address             `json:"token"`
	Operations     []BatchPaymentOperationData `json:"operations"`
	OperationsHash *string                     `json:"operations_hash"`
	BatchID        *string                     `json:"batch_id"`
	CreatedAt      uint64                      `json:"created_at"`
}

// TokenClawbackData is the decoded payload of a "TokenClawback" transaction —
// tokens reclaimed from an account back to a recipient.
type TokenClawbackData struct {
	From      common.Address `json:"from"`
	Recipient common.Address `json:"recipient"`
	Value     string         `json:"value"`
	Token     common.Address `json:"token"`
}

type TokenGrantAuthorityData struct {
	AuthorityAddress common.Address `json:"authority_address"`
	AuthorityType    AuthorityType  `json:"authority_type"`
	Token            common.Address `json:"token"`
	// Value is nil for authority types that do not carry an allowance (the node returns null).
	Value *string `json:"value"`
}

type TokenRevokeAuthorityData struct {
	AuthorityAddress common.Address `json:"authority_address"`
	AuthorityType    AuthorityType  `json:"authority_type"`
	Token            common.Address `json:"token"`
	// Value is nil for authority types that do not carry an allowance (the node returns null).
	Value *string `json:"value"`
}

type TokenBlacklistAccountData struct {
	Address common.Address `json:"address"`
	Token   common.Address `json:"token"`
}

type TokenWhitelistAccountData struct {
	Address common.Address `json:"address"`
	Token   common.Address `json:"token"`
}

type TokenMintData struct {
	Recipient common.Address `json:"recipient"`
	Token     common.Address `json:"token"`
	Value     string         `json:"value"`
}

type TokenBridgeAndMintData struct {
	Recipient      common.Address `json:"recipient"`
	Value          string         `json:"value"`
	SourceChainID  uint64         `json:"source_chain_id"`
	SourceTxHash   string         `json:"source_tx_hash"`
	BridgeMetadata string         `json:"bridge_metadata"`
	Token          common.Address `json:"token"`
}

type TokenBurnData struct {
	Value string         `json:"value"`
	Token common.Address `json:"token"`
}

type TokenBurnAndBridgeData struct {
	Value              string         `json:"value"`
	Sender             common.Address `json:"sender"`
	DestinationChainID uint64         `json:"destination_chain_id"`
	DestinationAddress string         `json:"destination_address"`
	EscrowFee          string         `json:"escrow_fee"`
	BridgeMetadata     string         `json:"bridge_metadata"`
	BridgeParam        string         `json:"bridge_param"`
	Token              common.Address `json:"token"`
}

type TokenCloseAccountData struct {
	Token common.Address `json:"token"`
}

type TokenPauseData struct {
	Token common.Address `json:"token"`
}

type TokenUnpauseData struct {
	Token common.Address `json:"token"`
}

type TransactionMetadataEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TransactionMetadata struct {
	Name               string                     `json:"name"`
	URI                string                     `json:"uri"`
	AdditionalMetadata []TransactionMetadataEntry `json:"additional_metadata"`
}

type TokenUpdateMetadataData struct {
	Metadata TransactionMetadata `json:"metadata"`
	Token    common.Address      `json:"token"`
}

// MultiSigSignerInfo is one signer in a decoded "CreateMultiSig" transaction.
// PublicKey is the node's 0x-hex compressed-key string (the read-side shape,
// distinct from the write-side MultiSigSigner whose PublicKey is raw bytes).
type MultiSigSignerInfo struct {
	PublicKey string `json:"public_key"`
	Weight    uint8  `json:"weight"`
}

// CreateMultiSigData is the decoded payload of a "CreateMultiSig" transaction.
// MultisigAddress is the derived account address the node computes from the
// signer set and threshold.
type CreateMultiSigData struct {
	Signers         []MultiSigSignerInfo `json:"signers"`
	Threshold       uint16               `json:"threshold"`
	MultisigAddress common.Address       `json:"multisig_address"`
}

type RawTransactionData struct {
	Input string         `json:"input"`
	Token common.Address `json:"token"`
}
