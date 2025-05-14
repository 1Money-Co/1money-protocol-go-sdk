package onemoney_test

import (
	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

func TestIssueToken(t *testing.T) {
	t.Logf("TestIssueToken started")
	var nonce uint64 = 0
	payload := onemoney.TokenIssuePayload{
		ChainID:         1212101,
		Decimals:        6,
		MasterAuthority: common.HexToAddress(onemoney.TestOperatorAddress),
		Name:            "1Money Stable Coin",
		Nonce:           nonce,
		Symbol:          "USD1",
	}
	client := onemoney.NewTestClient()
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
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
	result1, err1 := client.IssueToken(req)
	if err1 != nil {
		t.Fatalf("IssueToken failed: %v", err1)
	}
	t.Logf("Successfully issued token: %s", result1.Token)
	t.Logf("Transaction hash: %s", result1.Hash)
}

func TestGetTokenInfo(t *testing.T) {
	client := onemoney.NewTestClient()
	tokenAddress := onemoney.TestMintAccount
	result, err := client.GetTokenMetadata(tokenAddress)
	if err != nil {
		t.Fatalf("GetTokenMetadata failed: %v", err)
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
	client := onemoney.NewTestClient()
	msg := &UpdateMetadataMessage{
		ChainID:            1212101,
		Nonce:              0,
		Name:               "USDFF Stablecoin",
		URI:                "https://usdf.com",
		Token:              common.HexToAddress(onemoney.TestMintAccount),
		AdditionalMetadata: "[{\"key1\":\"v1\",\"key2\":\"v2\"}]",
	}
	signature, err := client.SignMessage(msg, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	req := &onemoney.UpdateMetadataRequest{
		ChainID:            1212101,
		Nonce:              0,
		Token:              onemoney.TestMintAccount,
		Name:               "USDFF Stablecoin",
		URI:                "https://usdf.com",
		AdditionalMetadata: "[{\"key1\":\"v1\",\"key2\":\"v2\"}]",
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}

	result, err := client.UpdateTokenMetadata(req)
	if err != nil {
		t.Fatalf("UpdateTokenMetadata failed: %v", err)
	}

	t.Log("\nMetadata Update Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestGrantMasterMintAuthority(t *testing.T) {
	client := onemoney.NewTestClient()
	var nonce uint64 = 0
	payload := onemoney.TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeMintTokens,
		AuthorityAddress: common.HexToAddress(onemoney.TestOperatorAddress),
		Token:            common.HexToAddress(onemoney.TestMintAccount),
		Value:            big.NewInt(1500000),
	}
	signature, err := client.SignMessage(payload, onemoney.Test2ndPrivateKey)
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
	result, err := client.GrantTokenAuthority(&req)
	if err != nil {
		t.Fatalf("GrantAuthority failed: %v", err)
	}
	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash:  %s", result.Hash)
}

func TestMintToken(t *testing.T) {
	client := onemoney.NewTestClient()
	// Get the current nonce
	var nonce uint64 = 0
	// Create mint payload
	payload := onemoney.TokenMintPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress(onemoney.TestOperatorAddress),
		Value:     big.NewInt(150000),
		Token:     common.HexToAddress(onemoney.TestMintAccount),
	}
	// Sign the payload
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
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
	result, err := client.MintToken(req)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}
	t.Log("\nMint Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}
