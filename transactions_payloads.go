package onemoney

import "github.com/ethereum/go-ethereum/common"

type TokenCreateData struct {
	Decimals        uint8          `json:"decimals"`
	IsPrivate       bool           `json:"is_private"`
	MasterAuthority common.Address `json:"master_authority"`
	Name            string         `json:"name"`
	Symbol          string         `json:"symbol"`
}

type TokenTransferData struct {
	Recipient common.Address `json:"recipient"`
	Token     common.Address `json:"token"`
	Value     string         `json:"value"`
}

type TokenGrantAuthorityData struct {
	AuthorityAddress common.Address `json:"authority_address"`
	AuthorityType    AuthorityType  `json:"authority_type"`
	Token            common.Address `json:"token"`
	Value            string         `json:"value"`
}

type TokenRevokeAuthorityData struct {
	AuthorityAddress common.Address `json:"authority_address"`
	AuthorityType    AuthorityType  `json:"authority_type"`
	Token            common.Address `json:"token"`
	Value            string         `json:"value"`
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
	BridgeMetadata string         `json:"bridge_metadata"`
	Recipient      common.Address `json:"recipient"`
	SourceChainID  uint64         `json:"source_chain_id"`
	SourceTxHash   string         `json:"source_tx_hash"`
	Token          common.Address `json:"token"`
	Value          string         `json:"value"`
}

type TokenBurnData struct {
	Recipient common.Address `json:"recipient"`
	Token     common.Address `json:"token"`
	Value     string         `json:"value"`
}

type TokenBurnAndBridgeData struct {
	BridgeMetadata     string         `json:"bridge_metadata"`
	DestinationAddress string         `json:"destination_address"`
	DestinationChainID uint64         `json:"destination_chain_id"`
	EscrowFee          string         `json:"escrow_fee"`
	Sender             common.Address `json:"sender"`
	Token              common.Address `json:"token"`
	Value              string         `json:"value"`
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

type RawTransactionData struct {
	Input string         `json:"input"`
	Token common.Address `json:"token"`
}

var defaultTransactionPayloadConstructors = map[TransactionType]func() TransactionPayload{
	TransactionTypeTokenCreate:           func() TransactionPayload { return &TokenCreateData{} },
	TransactionTypeTokenTransfer:         func() TransactionPayload { return &TokenTransferData{} },
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
	TransactionTypeRaw:                   func() TransactionPayload { return &RawTransactionData{} },
}

func init() {
	for tt, ctor := range defaultTransactionPayloadConstructors {
		RegisterTransactionPayload(tt, ctor)
	}
}

// isTransactionPayload is a compile-time marker; it intentionally has no runtime behavior.
func (*TokenCreateData) isTransactionPayload()           {}
func (*TokenTransferData) isTransactionPayload()         {}
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
