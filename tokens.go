package onemoney

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type TokenIssuePayload struct {
	ChainID         uint64         `json:"chain_id"`
	Nonce           uint64         `json:"nonce"`
	Symbol          string         `json:"symbol"`
	Name            string         `json:"name"`
	Decimals        uint8          `json:"decimals"`
	MasterAuthority common.Address `json:"master_authority"`
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
	BlackList               []string          `json:"black_list"`
	BlackListAuthorities    []string          `json:"black_list_authorities"`
	BurnAuthorities         []string          `json:"burn_authorities"`
	Decimals                uint8             `json:"decimals"`
	IsPaused                bool              `json:"is_paused"`
	MasterAuthority         string            `json:"master_authority"`
	MasterMintAuthority     string            `json:"master_mint_authority"`
	Meta                    Meta              `json:"meta"`
	MetadataUpdateAuthority string            `json:"metadata_update_authority"`
	MinterAuthorities       []MinterAuthority `json:"minter_authorities"`
	PauseAuthority          string            `json:"pause_authority"`
	Supply                  string            `json:"supply"`
	Symbol                  string            `json:"symbol"`
}

type UpdateMetadataRequest struct {
	AdditionalMetadata string    `json:"additional_metadata"`
	ChainID            int       `json:"chain_id"`
	Name               string    `json:"name"`
	Nonce              int       `json:"nonce"`
	Token              string    `json:"token"`
	URI                string    `json:"uri"`
	Signature          Signature `json:"signature"`
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
	AuthorityTypeMasterMint     AuthorityType = "MasterMint"
	AuthorityTypeMintTokens     AuthorityType = "MintTokens"
	AuthorityTypePause          AuthorityType = "Pause"
	AuthorityTypeBurn           AuthorityType = "BurnFromAccount"
	AuthorityTypeBlacklist      AuthorityType = "BlacklistAccount"
	AuthorityTypeUpdateMetadata AuthorityType = "UpdateMetadata"
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

type TokenAuthorityRequest struct {
	TokenAuthorityPayload
	Signature Signature `json:"signature"`
}

type GrantAuthorityResponse struct {
	Hash string `json:"hash"`
}

type TokenMintPayload struct {
	ChainID   uint64         `json:"chain_id"`
	Nonce     uint64         `json:"nonce"`
	Recipient common.Address `json:"recipient"`
	Value     *big.Int       `json:"value"`
	Token     common.Address `json:"token"`
}

type MintTokenRequest struct {
	TokenMintPayload
	Signature Signature `json:"signature"`
}

type MintTokenResponse struct {
	Hash string `json:"hash"`
}

type TokenBurnPayload struct {
	ChainID   uint64         `json:"chain_id"`
	Nonce     uint64         `json:"nonce"`
	Recipient common.Address `json:"recipient"`
	Value     *big.Int       `json:"value"`
	Token     common.Address `json:"token"`
}

type BurnTokenRequest struct {
	TokenBurnPayload
	Signature Signature `json:"signature"`
}

type BurnTokenResponse struct {
	Hash string `json:"hash"`
}

type TokenBlacklistPayload struct {
	ChainID uint64         `json:"chain_id"`
	Nonce   uint64         `json:"nonce"`
	Action  string         `json:"action"`
	Address common.Address `json:"address"`
	Token   common.Address `json:"token"`
}

type SetTokenBlacklistRequest struct {
	TokenBlacklistPayload
	Signature Signature `json:"signature"`
}

type SetTokenBlacklistResponse struct {
	Hash string `json:"hash"`
}

type PauseTokenPayload struct {
	ChainID uint64         `json:"chain_id"`
	Nonce   uint64         `json:"nonce"`
	Action  string         `json:"action"`
	Token   common.Address `json:"token"`
}

type PauseTokenRequest struct {
	PauseTokenPayload
	Signature Signature `json:"signature"`
}

type PauseTokenResponse struct {
	Hash string `json:"hash"`
}

func (api *Client) IssueToken(req *IssueTokenRequest) (*IssueTokenResponse, error) {
	result := new(IssueTokenResponse)
	return result, api.PostMethod("/v1/tokens/issue", req, result)
}

func (api *Client) GetTokenMetadata(tokenAddress string) (*TokenInfoResponse, error) {
	result := new(TokenInfoResponse)
	return result, api.GetMethod(fmt.Sprintf("/v1/tokens/token_metadata?token=%s", tokenAddress), result)
}

func (api *Client) UpdateTokenMetadata(req *UpdateMetadataRequest) (*UpdateMetadataResponse, error) {
	result := new(UpdateMetadataResponse)
	return result, api.PostMethod("/v1/tokens/update_metadata", req, result)
}

func (api *Client) GrantTokenAuthority(req *TokenAuthorityRequest) (*GrantAuthorityResponse, error) {
	result := new(GrantAuthorityResponse)
	return result, api.PostMethod("/v1/tokens/grant_authority", req, result)
}

func (api *Client) MintToken(req *MintTokenRequest) (*MintTokenResponse, error) {
	result := new(MintTokenResponse)
	return result, api.PostMethod("/v1/tokens/mint", req, result)
}

func (api *Client) BurnToken(req *BurnTokenRequest) (*BurnTokenResponse, error) {
	result := new(BurnTokenResponse)
	return result, api.PostMethod("/v1/tokens/burn", req, result)
}

func (api *Client) SetTokenBlacklist(req *SetTokenBlacklistRequest) (*SetTokenBlacklistResponse, error) {
	result := new(SetTokenBlacklistResponse)
	return result, api.PostMethod("/v1/tokens/blacklist", req, result)
}
func (api *Client) PauseToken(req *PauseTokenRequest) (*PauseTokenResponse, error) {
	result := new(PauseTokenResponse)
	return result, api.PostMethod("/v1/tokens/pause", req, result)
}

