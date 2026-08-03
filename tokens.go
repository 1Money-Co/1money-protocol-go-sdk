package onemoney

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
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

// Deprecated: use Tokens().Issue, which signs internally with a Signer and
// defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) IssueToken(ctx context.Context, req *IssueTokenRequest) (*IssueTokenResponse, error) {
	result := new(IssueTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensIssue, req, result)
}

// getTokenMetadata is the shared implementation behind the deprecated
// Client.GetTokenMetadata and the namespace Tokens().Metadata.
func (client *Client) getTokenMetadata(ctx context.Context, token string) (*TokenInfoResponse, error) {
	result := new(TokenInfoResponse)
	params := url.Values{}
	params.Set("token", token)
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTokensMetadata, params.Encode()), result)
}

// Deprecated: use Tokens().Metadata, which takes a typed common.Address. This
// method still works, unchanged.
func (client *Client) GetTokenMetadata(ctx context.Context, tokenAddress string) (*TokenInfoResponse, error) {
	return client.getTokenMetadata(ctx, tokenAddress)
}

// Deprecated: use Tokens().UpdateMetadata, which signs internally with a Signer
// and defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) UpdateTokenMetadata(ctx context.Context, req *UpdateMetadataRequest) (*UpdateMetadataResponse, error) {
	result := new(UpdateMetadataResponse)
	return result, client.PostMethod(ctx, endpointTokensUpdateMetadata, req, result)
}

// Deprecated: use Tokens().GrantAuthority or Tokens().RevokeAuthority, which
// sign internally with a Signer and default to domain-separated v2. This posts
// a legacy v1 request.
func (client *Client) GrantTokenAuthority(ctx context.Context, req *TokenAuthorityRequest) (*GrantAuthorityResponse, error) {
	result := new(GrantAuthorityResponse)
	return result, client.PostMethod(ctx, endpointTokensGrantAuthority, req, result)
}

// Deprecated: use Tokens().Mint, which signs internally with a Signer and
// defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) MintToken(ctx context.Context, req *MintTokenRequest) (*MintTokenResponse, error) {
	result := new(MintTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensMint, req, result)
}

// Deprecated: use Tokens().BridgeAndMint, which signs internally with a Signer
// and defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) BridgeAndMintToken(ctx context.Context, req *BridgeAndMintTokenRequest) (*BridgeAndMintTokenResponse, error) {
	result := new(BridgeAndMintTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensBridgeAndMint, req, result)
}

// Deprecated: use Tokens().Burn, which signs internally with a Signer and
// defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) BurnToken(ctx context.Context, req *BurnTokenRequest) (*BurnTokenResponse, error) {
	result := new(BurnTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensBurn, req, result)
}

// Deprecated: use Tokens().BurnAndBridge, which signs internally with a Signer
// and defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) BurnAndBridgeToken(ctx context.Context, req *BurnAndBridgeTokenRequest) (*BurnAndBridgeTokenResponse, error) {
	result := new(BurnAndBridgeTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensBurnAndBridge, req, result)
}

// Deprecated: use Tokens().ManageBlacklist, which signs internally with a
// Signer and defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) SetTokenBlacklist(ctx context.Context, req *SetTokenManageListRequest) (*SetTokenManageListResponse, error) {
	result := new(SetTokenManageListResponse)
	return result, client.PostMethod(ctx, endpointTokensManageBlacklist, req, result)
}

// Deprecated: use Tokens().ManageWhitelist, which signs internally with a
// Signer and defaults to domain-separated v2. This posts a legacy v1 request.
func (client *Client) SetTokenWhitelist(ctx context.Context, req *SetTokenManageListRequest) (*SetTokenManageListResponse, error) {
	result := new(SetTokenManageListResponse)
	return result, client.PostMethod(ctx, endpointTokensManageWhitelist, req, result)
}

// Deprecated: use Tokens().Pause or Tokens().Unpause, which sign internally with
// a Signer and default to domain-separated v2. This posts a legacy v1 request.
func (client *Client) PauseToken(ctx context.Context, req *PauseTokenRequest) (*PauseTokenResponse, error) {
	result := new(PauseTokenResponse)
	return result, client.PostMethod(ctx, endpointTokensPause, req, result)
}

// DeriveTokenAccountAddress derives the token account address given the wallet address and mint address.
//
// Address is 20 byte, 160 bits. Let's say if we want to support 50 billion accounts on 1money.
// That's about 36 bits. There are 124 bits remaining. In other words, the collision probability
// is 1/2^124, which is negligible. We therefore use the keccak256 hash of the wallet and mint address.
// func (client *Client) DeriveTokenAccountAddress(walletAddress common.Address, mintAddress common.Address) common.Address {
// 	buf := append(walletAddress.Bytes(), mintAddress.Bytes()...)
// 	hash := crypto.Keccak256(buf)
// 	return common.BytesToAddress(hash[12:])
// }

// -----------------------------------------------------------------------------
// Domain-separated v2 token writes (namespace API)
// -----------------------------------------------------------------------------

// TokensAPI groups the token operation methods: the domain-separated v2 submit
// methods (signing detail hidden — pass a payload and a Signer; v2 by default,
// WithLegacyV1 on the client selects /v1) plus token reads.
type TokensAPI struct{ c *Client }

// Tokens returns the token operation namespace.
func (c *Client) Tokens() TokensAPI { return TokensAPI{c: c} }

// Metadata fetches a token's on-chain info (symbol, authorities, supply, and
// so on) by token address.
func (a TokensAPI) Metadata(ctx context.Context, token common.Address) (*TokenInfoResponse, error) {
	return a.c.getTokenMetadata(ctx, token.Hex())
}

// Issue creates a new token. The response carries the transaction hash and the
// minted token address.
func (a TokensAPI) Issue(ctx context.Context, payload TokenIssuePayload, signer Signer, opts ...SubmitOption) (*IssueTokenResponse, error) {
	out := new(IssueTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// Mint mints tokens to a recipient.
func (a TokensAPI) Mint(ctx context.Context, payload TokenMintPayload, signer Signer, opts ...SubmitOption) (*MintTokenResponse, error) {
	out := new(MintTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// Burn burns tokens.
func (a TokensAPI) Burn(ctx context.Context, payload TokenBurnPayload, signer Signer, opts ...SubmitOption) (*BurnTokenResponse, error) {
	out := new(BurnTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// BridgeAndMint mints tokens bridged in from another chain.
func (a TokensAPI) BridgeAndMint(ctx context.Context, payload TokenBridgeAndMintPayload, signer Signer, opts ...SubmitOption) (*BridgeAndMintTokenResponse, error) {
	out := new(BridgeAndMintTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// BurnAndBridge burns tokens and bridges them to another chain.
func (a TokensAPI) BurnAndBridge(ctx context.Context, payload TokenBurnAndBridgePayload, signer Signer, opts ...SubmitOption) (*BurnAndBridgeTokenResponse, error) {
	out := new(BurnAndBridgeTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// GrantAuthority grants a token authority. The payload's Action is forced to
// "Grant".
func (a TokensAPI) GrantAuthority(ctx context.Context, payload TokenAuthorityPayload, signer Signer, opts ...SubmitOption) (*GrantAuthorityResponse, error) {
	payload.Action = AuthorityActionGrant
	out := new(GrantAuthorityResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// RevokeAuthority revokes a token authority. The payload's Action is forced to
// "Revoke". It shares the grant_authority endpoint.
func (a TokensAPI) RevokeAuthority(ctx context.Context, payload TokenAuthorityPayload, signer Signer, opts ...SubmitOption) (*GrantAuthorityResponse, error) {
	payload.Action = AuthorityActionRevoke
	out := new(GrantAuthorityResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// Clawback reclaims tokens from an account.
func (a TokensAPI) Clawback(ctx context.Context, payload TokenClawbackPayload, signer Signer, opts ...SubmitOption) (*BurnTokenResponse, error) {
	out := new(BurnTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// ManageBlacklist adds or removes a blacklist entry. Set payload.Action to
// "Add" or "Remove".
func (a TokensAPI) ManageBlacklist(ctx context.Context, payload TokenManageListPayload, signer Signer, opts ...SubmitOption) (*SetTokenManageListResponse, error) {
	cfg := resolveSubmit(opts)
	kind := ManageListBlacklist
	cfg.listKind = &kind
	out := new(SetTokenManageListResponse)
	return out, a.c.submitPayload(ctx, payload, cfg, signer, out)
}

// ManageWhitelist adds or removes a whitelist entry. Set payload.Action to
// "Add" or "Remove".
func (a TokensAPI) ManageWhitelist(ctx context.Context, payload TokenManageListPayload, signer Signer, opts ...SubmitOption) (*SetTokenManageListResponse, error) {
	cfg := resolveSubmit(opts)
	kind := ManageListWhitelist
	cfg.listKind = &kind
	out := new(SetTokenManageListResponse)
	return out, a.c.submitPayload(ctx, payload, cfg, signer, out)
}

// Pause pauses a token. The payload's Action is forced to "Pause".
func (a TokensAPI) Pause(ctx context.Context, payload PauseTokenPayload, signer Signer, opts ...SubmitOption) (*PauseTokenResponse, error) {
	payload.Action = Pause
	out := new(PauseTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// Unpause unpauses a token. The payload's Action is forced to "Unpause". It
// shares the pause endpoint.
func (a TokensAPI) Unpause(ctx context.Context, payload PauseTokenPayload, signer Signer, opts ...SubmitOption) (*PauseTokenResponse, error) {
	payload.Action = UnPause
	out := new(PauseTokenResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}

// UpdateMetadata updates a token's metadata.
func (a TokensAPI) UpdateMetadata(ctx context.Context, payload UpdateMetadataPayload, signer Signer, opts ...SubmitOption) (*UpdateMetadataResponse, error) {
	out := new(UpdateMetadataResponse)
	return out, a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out)
}
