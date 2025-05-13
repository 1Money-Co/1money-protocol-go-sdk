package onemoney_test

import (
	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
	"testing"
)

func TestIssueToken(t *testing.T) {
	t.Logf("TestIssueToken started")
	var nonce uint64 = 0
	payload := onemoney.TokenIssuePayload{
		ChainID:         1212101,
		Decimals:        6,
		MasterAuthority: common.HexToAddress(TestOperatorAddress),
		Name:            "1Money Stable Coin",
		Nonce:           nonce,
		Symbol:          "USD1",
	}
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	privateKey := strings.TrimPrefix(TestOperatorPrivateKey, "0x")
	signature, err := api.SignMessage(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	req := &onemoney.IssueTokenRequest{
		TokenIssuePayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	result1, err1 := api.IssueToken(req)
	if err1 != nil {
		t.Fatalf("IssueToken failed: %v", err1)
	}
	t.Logf("Successfully issued token: %s", result1.Token)
	t.Logf("Transaction hash: %s", result1.Hash)
}

func TestGetTokenInfo(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	tokenAddress := MintAccount
	result, err := api.GetTokenInfo(tokenAddress)
	if err != nil {
		t.Fatalf("GetTokenInfo failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	t.Log("\nToken Information:")
	t.Log("==================")
	t.Logf("Basic Info:")
	t.Logf("  Symbol:    %s", result.Symbol)
	t.Logf("  Decimals:  %d", result.Decimals)
	t.Logf("  Supply:    %s", result.Supply)
	t.Logf("  Is Paused: %v", result.IsPaused)

	t.Log("\nMeta:")
	t.Logf("  Name: %s", result.Meta.Name)
	t.Logf("  URI:  %s", result.Meta.URI)
	t.Log("  Additional Metadata:")
	for _, meta := range result.Meta.AdditionalMetadata {
		t.Logf("    %s: %s", meta.Key, meta.Value)
	}

	t.Log("\nAuthorities:")
	t.Logf("  Master:              %s", result.MasterAuthority)
	t.Logf("  Master Mint:         %s", result.MasterMintAuthority)
	t.Logf("  Metadata Update:     %s", result.MetadataUpdateAuthority)
	t.Logf("  Pause:              %s", result.PauseAuthority)

	t.Log("\nMinter Authorities:")
	for _, minter := range result.MinterAuthorities {
		t.Logf("  Minter: %s", minter.Minter)
		t.Logf("    Allowance: %s", minter.Allowance)
	}
	t.Log("\nOther Authorities:")
	t.Log("  Black List Authorities:")
	for _, auth := range result.BlackListAuthorities {
		t.Logf("    %s", auth)
	}
	t.Log("  Burn Authorities:")
	for _, auth := range result.BurnAuthorities {
		t.Logf("    %s", auth)
	}

	t.Log("\nBlack List:")
	for _, addr := range result.BlackList {
		t.Logf("  %s", addr)
	}
}

type UpdateMetadataMessage struct {
	ChainID            uint64
	Nonce              uint64
	Name               string
	URI                string
	Token              common.Address
	AdditionalMetadata string
}

func TestUpdateTokenMetadata(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	msg := &UpdateMetadataMessage{
		ChainID:            1212101,
		Nonce:              0,
		Name:               "USDFF Stablecoin",
		URI:                "https://usdf.com",
		Token:              common.HexToAddress(MintAccount),
		AdditionalMetadata: "[{\"key1\":\"v1\",\"key2\":\"v2\"}]",
	}
	privateKey := TestOperatorPrivateKey
	signature, err := api.SignMessage(msg, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	req := &onemoney.UpdateMetadataRequest{
		ChainID:            1212101,
		Nonce:              0,
		Token:              MintAccount,
		Name:               "USDFF Stablecoin",
		URI:                "https://usdf.com",
		AdditionalMetadata: "[{\"key1\":\"v1\",\"key2\":\"v2\"}]",
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}

	result, err := api.UpdateTokenMetadata(req)
	if err != nil {
		t.Fatalf("UpdateTokenMetadata failed: %v", err)
	}

	t.Log("\nMetadata Update Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestGrantMasterMintAuthority(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	var nonce uint64 = 0
	payload := onemoney.TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeMintTokens,
		AuthorityAddress: common.HexToAddress(TestOperatorAddress),
		Token:            common.HexToAddress(MintAccount),
		Value:            big.NewInt(1500000),
	}
	privateKey := strings.TrimPrefix(Test2ndPrivateKey, "0x")
	signature, err := api.SignMessage(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	t.Logf("\nGrant signature Result: %v", signature)
	req := onemoney.TokenAuthorityRequest{
		TokenAuthorityPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	result, err := api.GrantTokenAuthority(&req)
	if err != nil {
		t.Fatalf("GrantAuthority failed: %v", err)
	}
	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash:  %s", result.Hash)
}

func TestMintToken(t *testing.T) {
	api := onemoney.New(onemoney.ApiBaseUrlTest)
	// Get the current nonce
	var nonce uint64 = 0
	// Create mint payload
	payload := onemoney.TokenMintPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress(TestOperatorAddress),
		Value:     big.NewInt(150000),
		Token:     common.HexToAddress(MintAccount),
	}
	// Sign the payload
	privateKey := strings.TrimPrefix(TestOperatorPrivateKey, "0x")
	signature, err := api.SignMessage(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create mint request
	req := &onemoney.MintTokenRequest{
		TokenMintPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send mint request
	result, err := api.MintToken(req)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}
	t.Log("\nMint Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}
