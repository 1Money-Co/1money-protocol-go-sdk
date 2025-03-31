package tokens

import (
	"math/big"
	"strings"
	"testing"

	"go-1money/config"
	"go-1money/sign"

	"github.com/ethereum/go-ethereum/common"
)

func TestIssueToken(t *testing.T) {

	var nonce uint64 = 3

	payload := TokenIssuePayload{
		ChainID:         1212101,
		Decimals:        6,
		MasterAuthority: common.HexToAddress(config.MasterAuthorityAddress),
		Name:            "Palisade Testing Stablecoin",
		Nonce:           nonce,
		Symbol:          "USDPX",
	}

	privateKey := strings.TrimPrefix(config.OperatorPrivateKey, "0x")
	signature, err := sign.Message(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

	req := &IssueTokenRequest{
		TokenIssuePayload: payload,
		Signature: Signature{
			R: signature.R,
			S: signature.S,
			V: int(signature.V),
		},
	}

	result1, err1 := IssueToken(req)
	if err1 != nil {
		t.Fatalf("IssueToken failed: %v", err1)
	}

	t.Logf("Successfully issued token: %s", result1.Token)
	t.Logf("Transaction hash: %s", result1.Hash)
}

func TestGetTokenInfo(t *testing.T) {
	tokenAddress := "0x77be73b6e864221d2746b70982c299f60fd840cc"
	result, err := GetTokenInfo(tokenAddress)
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
	msg := &UpdateMetadataMessage{
		ChainID:            1212101,
		Nonce:              0,
		Name:               "USDFF Stablecoin",
		URI:                "https://usdf.com",
		Token:              common.HexToAddress("0x57a7d1514bae23bfc4e03dbb839e1ae2a2f18192"),
		AdditionalMetadata: "[{\"key1\":\"v1\",\"key2\":\"v2\"}]",
	}

	privateKey := "b1c49ed15a19a21541cd71a0837c75194756cbe81ac13c14e31213d766e84e7a"
	signature, err := sign.Message(msg, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

	req := &UpdateMetadataRequest{
		ChainID:            1212101,
		Nonce:              0,
		Token:              "0x57a7d1514bae23bfc4e03dbb839e1ae2a2f18192",
		Name:               "USDFF Stablecoin",
		URI:                "https://usdf.com",
		AdditionalMetadata: "[{\"key1\":\"v1\",\"key2\":\"v2\"}]",
		Signature: Signature{
			R: signature.R,
			S: signature.S,
			V: int(signature.V),
		},
	}

	result, err := UpdateTokenMetadata(req)
	if err != nil {
		t.Fatalf("UpdateTokenMetadata failed: %v", err)
	}

	t.Log("\nMetadata Update Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestGrantMasterMintAuthority(t *testing.T) {

	var nonce uint64 = 0

	payload := TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            nonce,
		Action:           AuthorityActionGrant,
		AuthorityType:    AuthorityTypeMintTokens,
		AuthorityAddress: common.HexToAddress(config.MintAuthorityAddress),
		Token:            common.HexToAddress("0x91f66cb6c9b56c7e3bcdb9eff9da13da171e89f4"),
		Value:            big.NewInt(1500000),
	}

	privateKey := strings.TrimPrefix(config.MasterAuthorityPrivateKey, "0x")
	signature, err := sign.Message(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

	t.Logf("\nGrant signature Result: %v", signature)

	req := TokenAuthorityRequest{
		TokenAuthorityPayload: payload,
		Signature: Signature{
			R: signature.R,
			S: signature.S,
			V: int(signature.V),
		},
	}

	result, err := GrantAuthority(&req)
	if err != nil {
		t.Fatalf("GrantAuthority failed: %v", err)
	}

	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash:  %s", result.Hash)
}

func TestMintToken(t *testing.T) {
	// Get the current nonce
	var nonce uint64 = 0
	// Create mint payload
	payload := TokenMintPayload{
		ChainID:   1212101,
		Nonce:     nonce,
		Recipient: common.HexToAddress(config.BurnAuthorityAddress),
		Value:     big.NewInt(150000),
		Token:     common.HexToAddress("0x91f66cb6c9b56c7e3bcdb9eff9da13da171e89f4"),
	}

	// Sign the payload
	privateKey := strings.TrimPrefix(config.MintAuthorityPrivateKey, "0x")
	signature, err := sign.Message(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

	// Create mint request
	req := &MintTokenRequest{
		TokenMintPayload: payload,
		Signature: Signature{
			R: signature.R,
			S: signature.S,
			V: int(signature.V),
		},
	}

	// Send mint request
	result, err := MintToken(req)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}

	t.Log("\nMint Token Result:")
	t.Log("=================")
	t.Logf("Transaction Hash: %s", result.Hash)
}
