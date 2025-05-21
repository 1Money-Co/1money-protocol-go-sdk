package onemoney_test

import (
	"context"
	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

func TestIssueToken(t *testing.T) {
	t.Logf("TestIssueToken started")
	var nonce uint64 = 81
	payload := onemoney.TokenIssuePayload{
		ChainID:         1212101,
		Decimals:        6,
		MasterAuthority: common.HexToAddress(onemoney.TestOperatorAddress),
		Name:            "1Money Stable Coin Aaron2",
		Nonce:           nonce,
		Symbol:          "USDA2",
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
	result, err := client.IssueToken(context.Background(), req)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}
	t.Logf("Successfully issued token: %s", result.Token)
	t.Logf("Transaction hash: %s", result.Hash)
}

func TestGetTokenInfo(t *testing.T) {
	client := onemoney.NewTestClient()
	tokenAddress := onemoney.TestTokenAddress
	result, err := client.GetTokenMetadata(context.Background(), tokenAddress)
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
	for _, minter := range result.MinterBurnAuthorities {
		t.Logf("  Minter: %s", minter.Minter)
		t.Logf("  Allowance: %s", minter.Allowance)
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

func TestUpdateTokenMetadata(t *testing.T) {
	client := onemoney.NewTestClient()
	payload := onemoney.UpdateMetadataPayload{
		ChainID: 1212101,
		Nonce:   81,
		Name:    "USDFF Stablecoin",
		URI:     "https://usdf.com",
		Token:   common.HexToAddress(onemoney.TestTokenAddress),
		AdditionalMetadata: []onemoney.AdditionalMetadata{
			{
				Key:   "test",
				Value: "test",
			},
		},
	}
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	req := &onemoney.UpdateMetadataRequest{
		UpdateMetadataPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	result, err := client.UpdateTokenMetadata(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateTokenMetadata failed: %v", err)
	}
	t.Log("\nMetadata Update Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestGrantMintBurnAuthority(t *testing.T) {
	client := onemoney.NewTestClient()
	var nonce uint64 = 15
	payload := onemoney.TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeMintBurnTokens,
		AuthorityAddress: common.HexToAddress(onemoney.TestOperatorAddress),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
		Value:            big.NewInt(1500000),
	}
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
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
	result, err := client.GrantTokenAuthority(context.Background(), &req)
	if err != nil {
		t.Fatalf("GrantAuthority failed: %v", err)
	}
	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash:  %s", result.Hash)
}

func TestGrantMasterMintAuthority(t *testing.T) {
	client := onemoney.NewTestClient()
	var nonce uint64 = 21
	payload := onemoney.TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeMasterMint,
		AuthorityAddress: common.HexToAddress(onemoney.TestOperatorAddress),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
		Value:            big.NewInt(1500000),
	}
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
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
	result, err := client.GrantTokenAuthority(context.Background(), &req)
	if err != nil {
		t.Fatalf("GrantAuthority failed: %v", err)
	}
	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash:  %s", result.Hash)
}

func TestGrantMasterUpdateMetadata(t *testing.T) {
	client := onemoney.NewTestClient()
	var nonce uint64 = 13
	payload := onemoney.TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeUpdateMetadata,
		AuthorityAddress: common.HexToAddress(onemoney.TestOperatorAddress),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
		Value:            big.NewInt(1500000),
	}
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
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
	result, err := client.GrantTokenAuthority(context.Background(), &req)
	if err != nil {
		t.Fatalf("GrantAuthority failed: %v", err)
	}
	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash:  %s", result.Hash)
}

func TestGrantMasterUpdatePause(t *testing.T) {
	client := onemoney.NewTestClient()
	var nonce uint64 = 16
	payload := onemoney.TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypePause,
		AuthorityAddress: common.HexToAddress(onemoney.TestOperatorAddress),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
		Value:            big.NewInt(1500000),
	}
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
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
	result, err := client.GrantTokenAuthority(context.Background(), &req)
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
	var nonce uint64 = 19
	// Create mint payload
	payload := onemoney.TokenMintPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress(onemoney.TestOperatorAddress),
		Value:     big.NewInt(150000),
		Token:     common.HexToAddress(onemoney.TestTokenAddress),
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
	result, err := client.MintToken(context.Background(), req)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}
	t.Log("\nMint Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestBurnToken(t *testing.T) {
	client := onemoney.NewTestClient()
	// Get the current nonce
	var nonce uint64 = 20
	// Create mint payload
	payload := onemoney.TokenBurnPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress(onemoney.TestOperatorAddress),
		Value:     big.NewInt(15000),
		Token:     common.HexToAddress(onemoney.TestTokenAddress),
	}
	// Sign the payload
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create burn request
	req := &onemoney.BurnTokenRequest{
		TokenBurnPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send burn request
	result, err := client.BurnToken(context.Background(), req)
	if err != nil {
		t.Fatalf("BurnToken failed: %v", err)
	}
	t.Log("\nBurn Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestBlacklist(t *testing.T) {
	client := onemoney.NewTestClient()
	// Get the current nonce
	var nonce uint64 = 63
	// Create SetTokenBlacklist payload
	payload := onemoney.TokenBlacklistPayload{
		ChainID: 1212101,
		Nonce:   nonce,
		Action:  onemoney.Whitelist,
		Address: common.HexToAddress(onemoney.TestOperatorAddress),
		Token:   common.HexToAddress(onemoney.TestTokenAddress),
	}
	// Sign the payload
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create SetTokenBlacklist request
	req := &onemoney.SetTokenBlacklistRequest{
		TokenBlacklistPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send mint request
	result, err := client.SetTokenBlacklist(context.Background(), req)
	if err != nil {
		t.Fatalf("SetTokenBlacklist failed: %v", err)
	}
	t.Log("\nSetTokenBlacklist Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestPauseToken(t *testing.T) {
	client := onemoney.NewTestClient()
	// Get the current nonce
	var nonce uint64 = 18
	// Create mint payload
	payload := onemoney.PauseTokenPayload{
		ChainID: 1212101,
		Nonce:   nonce,
		Action:  onemoney.Pause, // Unpause
		Token:   common.HexToAddress(onemoney.TestTokenAddress),
	}
	// Sign the payload
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create mint request
	req := &onemoney.PauseTokenRequest{
		PauseTokenPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send mint request
	result, err := client.PauseToken(context.Background(), req)
	if err != nil {
		t.Fatalf("PauseToken failed: %v", err)
	}
	t.Log("\nPause Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestUnPauseToken(t *testing.T) {
	client := onemoney.NewTestClient()
	// Get the current nonce
	var nonce uint64 = 23
	// Create pause payload
	payload := onemoney.PauseTokenPayload{
		ChainID: 1212101,
		Nonce:   nonce,
		Action:  onemoney.UnPause,
		Token:   common.HexToAddress(onemoney.TestTokenAddress),
	}
	// Sign the payload
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create pause request
	req := &onemoney.PauseTokenRequest{
		PauseTokenPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send pause request
	result, err := client.PauseToken(context.Background(), req)
	if err != nil {
		t.Fatalf("PauseToken failed: %v", err)
	}
	t.Log("\nPause Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}
