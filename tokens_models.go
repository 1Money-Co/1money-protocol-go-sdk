package onemoney

import "github.com/ethereum/go-ethereum/common"

// Deprecated: legacy v1 request type. Use Tokens().Issue with a Signer, which
// signs internally and defaults to domain-separated v2.
type IssueTokenRequest struct {
	TokenIssuePayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r IssueTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenIssuePayload, r.Signature)
}

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

// Deprecated: legacy v1 request type. Use Tokens().UpdateMetadata with a Signer,
// which signs internally and defaults to domain-separated v2.
type UpdateMetadataRequest struct {
	UpdateMetadataPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r UpdateMetadataRequest) Hash() (common.Hash, error) {
	return Hash(r.UpdateMetadataPayload, r.Signature)
}

type UpdateMetadataResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().GrantAuthority or
// Tokens().RevokeAuthority with a Signer, which sign internally and default to
// domain-separated v2.
type TokenAuthorityRequest struct {
	TokenAuthorityPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r TokenAuthorityRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenAuthorityPayload, r.Signature)
}

type GrantAuthorityResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().Mint with a Signer, which
// signs internally and defaults to domain-separated v2.
type MintTokenRequest struct {
	TokenMintPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r MintTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenMintPayload, r.Signature)
}

type MintTokenResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().BridgeAndMint with a Signer,
// which signs internally and defaults to domain-separated v2.
type BridgeAndMintTokenRequest struct {
	TokenBridgeAndMintPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r BridgeAndMintTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenBridgeAndMintPayload, r.Signature)
}

type BridgeAndMintTokenResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().Burn with a Signer, which
// signs internally and defaults to domain-separated v2.
type BurnTokenRequest struct {
	TokenBurnPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r BurnTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenBurnPayload, r.Signature)
}

type BurnTokenResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().BurnAndBridge with a Signer,
// which signs internally and defaults to domain-separated v2.
type BurnAndBridgeTokenRequest struct {
	TokenBurnAndBridgePayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r BurnAndBridgeTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenBurnAndBridgePayload, r.Signature)
}

type BurnAndBridgeTokenResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().ManageBlacklist or
// Tokens().ManageWhitelist with a Signer, which sign internally and default to
// domain-separated v2.
type SetTokenManageListRequest struct {
	TokenManageListPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r SetTokenManageListRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenManageListPayload, r.Signature)
}

type SetTokenManageListResponse struct {
	Hash string `json:"hash"`
}

// Deprecated: legacy v1 request type. Use Tokens().Pause or Tokens().Unpause
// with a Signer, which sign internally and default to domain-separated v2.
type PauseTokenRequest struct {
	PauseTokenPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r PauseTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.PauseTokenPayload, r.Signature)
}

type PauseTokenResponse struct {
	Hash string `json:"hash"`
}
