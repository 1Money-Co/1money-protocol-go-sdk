package onemoney

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"math/big"
	"net/http"
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

type TokenInfo struct {
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

func (api *Client) IssueToken(req *IssueTokenRequest) (*IssueTokenResponse, error) {
	gin.SetMode(gin.ReleaseMode)
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	url := api.baseUrl + "/v1/tokens/issue"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to issue token: %w", err)
	}
	var result IssueTokenResponse
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *Client) GetTokenInfo(tokenAddress string) (*TokenInfo, error) {
	gin.SetMode(gin.ReleaseMode)
	url := fmt.Sprintf(api.baseUrl+"/v1/tokens/token_metadata?token=%s", tokenAddress)
	println("access url: ", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get token info: %w", err)
	}
	var result TokenInfo
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *Client) UpdateTokenMetadata(req *UpdateMetadataRequest) (*UpdateMetadataResponse, error) {
	gin.SetMode(gin.ReleaseMode)
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq, err :=
		http.NewRequest("POST", api.baseUrl+"/v1/tokens/update_metadata", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update token metadata: %w", err)
	}
	var result UpdateMetadataResponse
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *Client) GrantTokenAuthority(req *TokenAuthorityRequest) (*GrantAuthorityResponse, error) {
	gin.SetMode(gin.ReleaseMode)
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq, err := http.NewRequest("POST", api.baseUrl+"/v1/tokens/grant_authority", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to grant authority: %w", err)
	}
	var result GrantAuthorityResponse
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (api *Client) MintToken(req *MintTokenRequest) (*MintTokenResponse, error) {
	gin.SetMode(gin.ReleaseMode)
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq, err := http.NewRequest("POST", api.baseUrl+"/v1/tokens/mint", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to mint token: %w", err)
	}
	var result MintTokenResponse
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
