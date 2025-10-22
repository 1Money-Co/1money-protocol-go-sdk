package onemoney_test

import (
	"context"
	"math/big"
	"testing"

	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

func TestIssueToken(t *testing.T) {
	t.Logf("TestIssueToken started")
	client := onemoney.NewTestClient()
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.TokenIssuePayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Symbol:           "USDA",
		Name:             "1Money Stable Coin Aaron",
		Decimals:         6,
		MasterAuthority:  common.HexToAddress(onemoney.TestOperatorAddress),
		IsPrivate:        false,
	}
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
	t.Logf("  Master Mint:         %s", result.MasterMintBurnAuthority)
	t.Log("\nMint Burn Authorities:")
	for _, minter := range result.MintBurnAuthority {
		t.Logf("  Minter: %s", minter.Minter)
		t.Logf("  Allowance: %s", minter.Allowance)
	}
	t.Log("\nOther Authorities:")
	t.Log("\nBlack List Authorities:")
	for _, auth := range result.ListAuthorities {
		t.Logf("    %s", auth)
	}
	t.Log("\nPause Authorities:")
	for _, auth := range result.PauseAuthorities {
		t.Logf("    %s", auth)
	}
	t.Log("\nBlack List:")
	for _, addr := range result.BlackList {
		t.Logf("  %s", addr)
	}
	t.Log("\nWhite List:")
	for _, addr := range result.WhiteList {
		t.Logf("  %s", addr)
	}
	t.Log("\nMetadata Update Authorities:")
	for _, addr := range result.MetadataUpdateAuthorities {
		t.Logf("  %s", addr)
	}
}

func TestUpdateTokenMetadata(t *testing.T) {
	client := onemoney.NewTestClient()
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.UpdateMetadataPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Name:             "USDFF Stablecoin",
		URI:              "https://usdf.com",
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.TokenAuthorityPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.TokenAuthorityPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeMasterMintBurn,
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.TokenAuthorityPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.TokenAuthorityPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
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

func TestGrantManageListPause(t *testing.T) {
	client := onemoney.NewTestClient()
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	payload := onemoney.TokenAuthorityPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.AuthorityActionGrant,
		AuthorityType:    onemoney.AuthorityTypeManageList,
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	// Create mint payload
	payload := onemoney.TokenMintPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Recipient:        common.HexToAddress(onemoney.TestOperatorAddress),
		Value:            big.NewInt(150000),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	// Create burn payload
	payload := onemoney.TokenBurnPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Recipient:        common.HexToAddress(onemoney.TestOperatorAddress),
		Value:            big.NewInt(15000),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	// Create SetTokenManagelist payload
	payload := onemoney.TokenManageListPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.ManageListActionRemove,
		Address:          common.HexToAddress(onemoney.BlacklistAddress),
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
	}
	// Sign the payload
	signature, err := client.SignMessage(payload, onemoney.TestOperatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}
	// Create SetTokenManagelist request
	req := &onemoney.SetTokenManageListRequest{
		TokenManageListPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}
	// Send mint request
	result, err := client.SetTokenBlacklist(context.Background(), req)
	if err != nil {
		t.Fatalf("SetTokenManagelist failed: %v", err)
	}
	t.Log("\nSetTokenManagelist Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestPauseToken(t *testing.T) {
	client := onemoney.NewTestClient()
	// Get the current nonce
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	// Create pause payload
	payload := onemoney.PauseTokenPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.Pause, // Unpause
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
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
	accountNonce, err := client.GetAccountNonce(context.Background(), onemoney.TestOperatorAddress)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	var nonce uint64 = accountNonce.Nonce

	// Get latest checkpoint
	latestCheckpoint, err := client.GetCheckpointNumber(context.Background())
	if err != nil {
		t.Fatalf("Failed to get latest checkpoint number: %v", err)
	}

	// Create pause payload
	payload := onemoney.PauseTokenPayload{
		RecentCheckpoint: uint64(latestCheckpoint.Number),
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           onemoney.UnPause,
		Token:            common.HexToAddress(onemoney.TestTokenAddress),
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

func TestDeriveTokenAccountAddress(t *testing.T) {
	client := onemoney.NewTestClient()
	address := client.DeriveTokenAccountAddress(common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"), common.HexToAddress("0x8E9d1b45293e30EF38564582979195DD16A16E13"))
	t.Logf("address: %s", address)
}
