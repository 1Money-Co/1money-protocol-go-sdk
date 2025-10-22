package onemoney

import (
	"context"
	"fmt"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

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

type IssueTokenRequest struct {
	TokenIssuePayload
	Signature Signature `json:"signature"`
}

type IssueTokenResponse struct {
	Hash  string `json:"hash"`
	Token string `json:"token"`
}

type AdditionalMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Meta struct {
	AdditionalMetadata []AdditionalMetadata `json:"additional_metadata"`
	Name               string               `json:"name"`
	URI                string               `json:"uri"`
}

type MinterAuthority struct {
	Allowance string `json:"allowance"`
	Minter    string `json:"minter"`
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
	Supply                    string            `json:"supply"`
	Decimals                  uint8             `json:"decimals"`
	IsPaused                  bool              `json:"is_paused"`
	IsPrivate                 bool              `json:"is_private"`
	Meta                      Meta              `json:"meta"`
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

type UpdateMetadataRequest struct {
	UpdateMetadataPayload
	Signature Signature `json:"signature"`
}

type UpdateMetadataResponse struct {
	Hash string `json:"hash"`
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

type TokenAuthorityRequest struct {
	TokenAuthorityPayload
	Signature Signature `json:"signature"`
}

type GrantAuthorityResponse struct {
	Hash string `json:"hash"`
}

type TokenMintPayload struct {
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Recipient        common.Address `json:"recipient"`
	Value            *big.Int       `json:"value"`
	Token            common.Address `json:"token"`
}

type MintTokenRequest struct {
	TokenMintPayload
	Signature Signature `json:"signature"`
}

type MintTokenResponse struct {
	Hash string `json:"hash"`
}

type TokenBurnPayload struct {
	RecentCheckpoint uint64         `json:"recent_checkpoint"`
	ChainID          uint64         `json:"chain_id"`
	Nonce            uint64         `json:"nonce"`
	Recipient        common.Address `json:"recipient"`
	Value            *big.Int       `json:"value"`
	Token            common.Address `json:"token"`
}

type BurnTokenRequest struct {
	TokenBurnPayload
	Signature Signature `json:"signature"`
}

type BurnTokenResponse struct {
	Hash string `json:"hash"`
}

type TokenManageListPayload struct {
	RecentCheckpoint uint64               `json:"recent_checkpoint"`
	ChainID          uint64               `json:"chain_id"`
	Nonce            uint64               `json:"nonce"`
	Action           ManageListActionType `json:"action"`
	Address          common.Address       `json:"address"`
	Token            common.Address       `json:"token"`
}

type SetTokenManageListRequest struct {
	TokenManageListPayload
	Signature Signature `json:"signature"`
}

type SetTokenManageListResponse struct {
	Hash string `json:"hash"`
}

type PauseTokenPayload struct {
	RecentCheckpoint uint64          `json:"recent_checkpoint"`
	ChainID          uint64          `json:"chain_id"`
	Nonce            uint64          `json:"nonce"`
	Action           PauseActionType `json:"action"`
	Token            common.Address  `json:"token"`
}

type PauseTokenRequest struct {
	PauseTokenPayload
	Signature Signature `json:"signature"`
}

type PauseTokenResponse struct {
	Hash string `json:"hash"`
}

func (client *Client) IssueToken(ctx context.Context, req *IssueTokenRequest) (*IssueTokenResponse, error) {
	result := new(IssueTokenResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/issue", req, result)
}

func (client *Client) GetTokenMetadata(ctx context.Context, tokenAddress string) (*TokenInfoResponse, error) {
	result := new(TokenInfoResponse)
	params := url.Values{}
	params.Set("token", tokenAddress)
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/tokens/token_metadata?%s", params.Encode()), result)
}

func (client *Client) UpdateTokenMetadata(ctx context.Context, req *UpdateMetadataRequest) (*UpdateMetadataResponse, error) {
	result := new(UpdateMetadataResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/update_metadata", req, result)
}

func (client *Client) GrantTokenAuthority(ctx context.Context, req *TokenAuthorityRequest) (*GrantAuthorityResponse, error) {
	result := new(GrantAuthorityResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/grant_authority", req, result)
}

func (client *Client) MintToken(ctx context.Context, req *MintTokenRequest) (*MintTokenResponse, error) {
	result := new(MintTokenResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/mint", req, result)
}

func (client *Client) BurnToken(ctx context.Context, req *BurnTokenRequest) (*BurnTokenResponse, error) {
	result := new(BurnTokenResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/burn", req, result)
}

func (client *Client) SetTokenBlacklist(ctx context.Context, req *SetTokenManageListRequest) (*SetTokenManageListResponse, error) {
	result := new(SetTokenManageListResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/manage_blacklist", req, result)
}

func (client *Client) SetTokenWhitelist(ctx context.Context, req *SetTokenManageListRequest) (*SetTokenManageListResponse, error) {
	result := new(SetTokenManageListResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/manage_whitelist", req, result)
}

func (client *Client) PauseToken(ctx context.Context, req *PauseTokenRequest) (*PauseTokenResponse, error) {
	result := new(PauseTokenResponse)
	return result, client.PostMethod(ctx, "/v1/tokens/pause", req, result)
}

// DeriveTokenAccountAddress derives the token account address given the wallet address and mint address.
//
// Address is 20 byte, 160 bits. Let's say if we want to support 50 billion
// accounts on 1money. That's about 36 bits. There are 124 bits remaining. In
// other words, the collision probability is 1/2^124, which is very very low.
// So, we will be fine to just use the hash of the wallet address and mint
// address to derive the token account address.
func (client *Client) DeriveTokenAccountAddress(walletAddress common.Address, mintAddress common.Address) common.Address {
	// Concatenate wallet address and mint address bytes
	buf := append(walletAddress.Bytes(), mintAddress.Bytes()...)

	// Calculate keccak256 hash
	hash := crypto.Keccak256(buf)

	// Return the last 20 bytes of the hash as the token account address
	return common.BytesToAddress(hash[12:])
}
