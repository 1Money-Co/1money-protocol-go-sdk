package onemoney

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// -----------------------------------------------------------------------------
// Token Payload Types
// -----------------------------------------------------------------------------

type TokenIssuePayload struct {
	ChainID         uint64         `json:"chain_id"`
	Nonce           uint64         `json:"nonce"`
	Symbol          string         `json:"symbol"`
	Name            string         `json:"name"`
	Decimals        uint8          `json:"decimals"`
	MasterAuthority common.Address `json:"master_authority"`
	IsPrivate       bool           `json:"is_private"`
	ClawbackEnabled bool           `json:"clawback_enabled"`
}

type AdditionalMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UpdateMetadataPayload struct {
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
	AuthorityTypeClawback       AuthorityType = "Clawback"
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
	ChainID          uint64          `json:"chain_id"`
	Nonce            uint64          `json:"nonce"`
	Action           AuthorityAction `json:"action"`
	AuthorityType    AuthorityType   `json:"authority_type"`
	AuthorityAddress common.Address  `json:"authority_address"`
	Token            common.Address  `json:"token"`
	Value            *big.Int        `json:"value"`
}

type TokenMintPayload struct {
	ChainID   uint64         `json:"chain_id"`
	Nonce     uint64         `json:"nonce"`
	Recipient common.Address `json:"recipient"`
	Value     *big.Int       `json:"value"`
	Token     common.Address `json:"token"`
}

type TokenBridgeAndMintPayload struct {
	ChainID        uint64         `json:"chain_id"`
	Nonce          uint64         `json:"nonce"`
	Recipient      common.Address `json:"recipient"`
	Value          *big.Int       `json:"value"`
	Token          common.Address `json:"token"`
	SourceChainID  uint64         `json:"source_chain_id"`
	SourceTxHash   string         `json:"source_tx_hash"`
	BridgeMetadata string         `json:"bridge_metadata"`
}

type TokenBurnPayload struct {
	ChainID uint64         `json:"chain_id"`
	Nonce   uint64         `json:"nonce"`
	Value   *big.Int       `json:"value"`
	Token   common.Address `json:"token"`
}

// TokenClawbackPayload reclaims tokens from an account back to a recipient.
type TokenClawbackPayload struct {
	ChainID   uint64         `json:"chain_id"`
	Nonce     uint64         `json:"nonce"`
	Token     common.Address `json:"token"`
	From      common.Address `json:"from"`
	Recipient common.Address `json:"recipient"`
	Value     *big.Int       `json:"value"`
}

type TokenBurnAndBridgePayload struct {
	ChainID            uint64         `json:"chain_id"`
	Nonce              uint64         `json:"nonce"`
	Sender             common.Address `json:"sender"`
	Value              *big.Int       `json:"value"`
	Token              common.Address `json:"token"`
	DestinationChainID uint64         `json:"destination_chain_id"`
	DestinationAddress string         `json:"destination_address"`
	EscrowFee          *big.Int       `json:"escrow_fee"`
	BridgeMetadata     string         `json:"bridge_metadata"`
	BridgeParam        HexBytes       `json:"bridge_param"`
}

type TokenManageListPayload struct {
	ChainID uint64               `json:"chain_id"`
	Nonce   uint64               `json:"nonce"`
	Action  ManageListActionType `json:"action"`
	Address common.Address       `json:"address"`
	Token   common.Address       `json:"token"`
}

type PauseTokenPayload struct {
	ChainID uint64          `json:"chain_id"`
	Nonce   uint64          `json:"nonce"`
	Action  PauseActionType `json:"action"`
	Token   common.Address  `json:"token"`
}

// -----------------------------------------------------------------------------
// Token API responses (shared by the legacy and v2 submit methods)
// -----------------------------------------------------------------------------

type IssueTokenResponse struct {
	Hash  string `json:"hash"`
	Token string `json:"token"`
}

type TokenInfoResponse struct {
	Symbol                    string            `json:"symbol"`
	MasterAuthority           string            `json:"master_authority"`
	MasterMintBurnAuthority   string            `json:"master_mint_burn_authority"`
	MintBurnAuthority         []MinterAuthority `json:"mint_burn_authorities"`
	PauseAuthorities          []string          `json:"pause_authorities"`
	ListAuthorities           []string          `json:"list_authorities"`
	BlackList                 []string          `json:"black_list"`
	WhiteList                 []string          `json:"white_list"`
	MetadataUpdateAuthorities []string          `json:"metadata_update_authorities"`
	BridgeMintAuthorities     []string          `json:"bridge_mint_authorities"`
	Supply                    string            `json:"supply"`
	Decimals                  uint8             `json:"decimals"`
	IsPaused                  bool              `json:"is_paused"`
	IsPrivate                 bool              `json:"is_private"`
	Meta                      Meta              `json:"meta"`
}

type MinterAuthority struct {
	Allowance string `json:"allowance"`
	Minter    string `json:"minter"`
}

type Meta struct {
	AdditionalMetadata []AdditionalMetadata `json:"additional_metadata"`
	Name               string               `json:"name"`
	URI                string               `json:"uri"`
}

type UpdateMetadataResponse struct {
	Hash string `json:"hash"`
}

type GrantAuthorityResponse struct {
	Hash string `json:"hash"`
}

type MintTokenResponse struct {
	Hash string `json:"hash"`
}

type BridgeAndMintTokenResponse struct {
	Hash string `json:"hash"`
}

type BurnTokenResponse struct {
	Hash string `json:"hash"`
}

type BurnAndBridgeTokenResponse struct {
	Hash string `json:"hash"`
}

type SetTokenManageListResponse struct {
	Hash string `json:"hash"`
}

type PauseTokenResponse struct {
	Hash string `json:"hash"`
}
