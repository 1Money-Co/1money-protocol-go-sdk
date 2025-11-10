package onemoney

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// -----------------------------------------------------------------------------
// Token Payload Types
// -----------------------------------------------------------------------------

type TokenIssuePayload struct {
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Symbol           string         `json:"symbol"`
	Name             string         `json:"name"`
	Decimals         uint8          `json:"decimals"`
	MasterAuthority  common.Address `json:"master_authority"`
	IsPrivate        bool           `json:"is_private"`
}

type AdditionalMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UpdateMetadataPayload struct {
	RecentCheckpoint   uint64               `json:"recent_checkpoint"`
	ChainID            uint64               `json:"chain_id"`
	Nonce              uint64               `json:"nonce"`
	Name               string               `json:"name"`
	URI                string               `json:"uri"`
	Token              common.Address       `json:"token"`
	AdditionalMetadata []AdditionalMetadata `json:"additional_metadata"`
}

type AuthorityAction string

const (
	AuthorityActionGrant  AuthorityAction = "Grant"
	AuthorityActionRevoke AuthorityAction = "Revoke"
)

type AuthorityType string

const (
	AuthorityTypeMasterMintBurn AuthorityType = "MasterMintBurn"
	AuthorityTypeMintBurnTokens AuthorityType = "MintBurnTokens"
	AuthorityTypePause          AuthorityType = "Pause"
	AuthorityTypeManageList     AuthorityType = "ManageList"
	AuthorityTypeUpdateMetadata AuthorityType = "UpdateMetadata"
	AuthorityTypeBridge         AuthorityType = "Bridge"
)

type PauseActionType string

const (
	Pause   PauseActionType = "Pause"
	UnPause PauseActionType = "Unpause"
)

type ManageListActionType string

const (
	ManageListActionAdd    ManageListActionType = "Add"
	ManageListActionRemove ManageListActionType = "Remove"
)

type TokenAuthorityPayload struct {
	RecentCheckpoint uint64          `json:"recent_checkpoint"`
	ChainID          uint64          `json:"chain_id"`
	Nonce            uint64          `json:"nonce"`
	Action           AuthorityAction `json:"action"`
	AuthorityType    AuthorityType   `json:"authority_type"`
	AuthorityAddress common.Address  `json:"authority_address"`
	Token            common.Address  `json:"token"`
	Value            *big.Int        `json:"value"`
}

type TokenMintPayload struct {
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Recipient        common.Address `json:"recipient"`
	Value            *big.Int       `json:"value"`
	Token            common.Address `json:"token"`
}

type TokenBridgeAndMintPayload struct {
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Recipient        common.Address `json:"recipient"`
	Value            *big.Int       `json:"value"`
	Token            common.Address `json:"token"`
	SourceChainID    uint64         `json:"source_chain_id"`
	SourceTxHash     string         `json:"source_tx_hash"`
	BridgeMetadata   string         `json:"bridge_metadata"`
}

type TokenBurnPayload struct {
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Recipient        common.Address `json:"recipient"`
	Value            *big.Int       `json:"value"`
	Token            common.Address `json:"token"`
}

type TokenBurnAndBridgePayload struct {
	RecentCheckpoint   uint64         `json:"recent_checkpoint"`
	ChainID            uint64         `json:"chain_id"`
	Nonce              uint64         `json:"nonce"`
	Sender             common.Address `json:"sender"`
	Value              *big.Int       `json:"value"`
	Token              common.Address `json:"token"`
	DestinationChainID uint64         `json:"destination_chain_id"`
	DestinationAddress string         `json:"destination_address"`
	EscrowFee          *big.Int       `json:"escrow_fee"`
	BridgeMetadata     string         `json:"bridge_metadata"`
}

type TokenManageListPayload struct {
	RecentCheckpoint uint64               `json:"recent_checkpoint"`
	ChainID          uint64               `json:"chain_id"`
	Nonce            uint64               `json:"nonce"`
	Action           ManageListActionType `json:"action"`
	Address          common.Address       `json:"address"`
	Token            common.Address       `json:"token"`
}

type PauseTokenPayload struct {
	RecentCheckpoint uint64          `json:"recent_checkpoint"`
	ChainID          uint64          `json:"chain_id"`
	Nonce            uint64          `json:"nonce"`
	Action           PauseActionType `json:"action"`
	Token            common.Address  `json:"token"`
}
