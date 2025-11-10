package onemoney

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	endpointTokensIssue           = "/v1/tokens/issue"
	endpointTokensMetadata        = "/v1/tokens/token_metadata"
	endpointTokensUpdateMetadata  = "/v1/tokens/update_metadata"
	endpointTokensGrantAuthority  = "/v1/tokens/grant_authority"
	endpointTokensMint            = "/v1/tokens/mint"
	endpointTokensBridgeAndMint   = "/v1/tokens/bridge_and_mint"
	endpointTokensBurn            = "/v1/tokens/burn"
	endpointTokensBurnAndBridge   = "/v1/tokens/burn_and_bridge"
	endpointTokensManageBlacklist = "/v1/tokens/manage_blacklist"
	endpointTokensManageWhitelist = "/v1/tokens/manage_whitelist"
	endpointTokensPause           = "/v1/tokens/pause"
)

func (client *Client) IssueToken(ctx context.Context, req *IssueTokenRequest) (*IssueTokenResponse, error) {
	result := new(IssueTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensIssue, req, result)
}

func (client *Client) GetTokenMetadata(ctx context.Context, tokenAddress string) (*TokenInfoResponse, error) {
	result := new(TokenInfoResponse)
	params := url.Values{}
	params.Set("token", tokenAddress)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTokensMetadata, params.Encode()), result)
}

func (client *Client) UpdateTokenMetadata(ctx context.Context, req *UpdateMetadataRequest) (*UpdateMetadataResponse, error) {
	result := new(UpdateMetadataResponse)
	return result, client.PostMethod(ctx, endpointTokensUpdateMetadata, req, result)
}

func (client *Client) GrantTokenAuthority(ctx context.Context, req *TokenAuthorityRequest) (*GrantAuthorityResponse, error) {
	result := new(GrantAuthorityResponse)
	return result, client.PostMethod(ctx, endpointTokensGrantAuthority, req, result)
}

func (client *Client) MintToken(ctx context.Context, req *MintTokenRequest) (*MintTokenResponse, error) {
	result := new(MintTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensMint, req, result)
}

func (client *Client) BridgeAndMintToken(ctx context.Context, req *BridgeAndMintTokenRequest) (*BridgeAndMintTokenResponse, error) {
	result := new(BridgeAndMintTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensBridgeAndMint, req, result)
}

func (client *Client) BurnToken(ctx context.Context, req *BurnTokenRequest) (*BurnTokenResponse, error) {
	result := new(BurnTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensBurn, req, result)
}

func (client *Client) BurnAndBridgeToken(ctx context.Context, req *BurnAndBridgeTokenRequest) (*BurnAndBridgeTokenResponse, error) {
	result := new(BurnAndBridgeTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensBurnAndBridge, req, result)
}

func (client *Client) SetTokenBlacklist(ctx context.Context, req *SetTokenManageListRequest) (*SetTokenManageListResponse, error) {
	result := new(SetTokenManageListResponse)
	return result, client.PostMethod(ctx, endpointTokensManageBlacklist, req, result)
}

func (client *Client) SetTokenWhitelist(ctx context.Context, req *SetTokenManageListRequest) (*SetTokenManageListResponse, error) {
	result := new(SetTokenManageListResponse)
	return result, client.PostMethod(ctx, endpointTokensManageWhitelist, req, result)
}

func (client *Client) PauseToken(ctx context.Context, req *PauseTokenRequest) (*PauseTokenResponse, error) {
	result := new(PauseTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensPause, req, result)
}

// DeriveTokenAccountAddress derives the token account address given the wallet address and mint address.
//
// Address is 20 byte, 160 bits. Let's say if we want to support 50 billion accounts on 1money.
// That's about 36 bits. There are 124 bits remaining. In other words, the collision probability
// is 1/2^124, which is negligible. We therefore use the keccak256 hash of the wallet and mint address.
func (client *Client) DeriveTokenAccountAddress(walletAddress common.Address, mintAddress common.Address) common.Address {
	buf := append(walletAddress.Bytes(), mintAddress.Bytes()...)
	hash := crypto.Keccak256(buf)
	return common.BytesToAddress(hash[12:])
}
