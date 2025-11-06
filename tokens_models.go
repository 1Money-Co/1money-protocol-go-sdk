package onemoney

type IssueTokenRequest struct {
	TokenIssuePayload
	Signature Signature `json:"signature"`
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
	BridgeAuthorities         []string          `json:"bridge_authorities"`
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

type UpdateMetadataRequest struct {
	UpdateMetadataPayload
	Signature Signature `json:"signature"`
}

type UpdateMetadataResponse struct {
	Hash string `json:"hash"`
}

type TokenAuthorityRequest struct {
	TokenAuthorityPayload
	Signature Signature `json:"signature"`
}

type GrantAuthorityResponse struct {
	Hash string `json:"hash"`
}

type MintTokenRequest struct {
	TokenMintPayload
	Signature Signature `json:"signature"`
}

type MintTokenResponse struct {
	Hash string `json:"hash"`
}

type BridgeAndMintTokenRequest struct {
	TokenBridgeAndMintPayload
	Signature Signature `json:"signature"`
}

type BridgeAndMintTokenResponse struct {
	Hash string `json:"hash"`
}

type BurnTokenRequest struct {
	TokenBurnPayload
	Signature Signature `json:"signature"`
}

type BurnTokenResponse struct {
	Hash string `json:"hash"`
}

type BurnAndBridgeTokenRequest struct {
	TokenBurnAndBridgePayload
	Signature Signature `json:"signature"`
}

type BurnAndBridgeTokenResponse struct {
	Hash string `json:"hash"`
}

type SetTokenManageListRequest struct {
	TokenManageListPayload
	Signature Signature `json:"signature"`
}

type SetTokenManageListResponse struct {
	Hash string `json:"hash"`
}

type PauseTokenRequest struct {
	PauseTokenPayload
	Signature Signature `json:"signature"`
}

type PauseTokenResponse struct {
	Hash string `json:"hash"`
}
