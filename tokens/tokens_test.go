package tokens

import (
	"math/big"
	"testing"

	"go-1money/sign"

	"github.com/ethereum/go-ethereum/common"
)

type IssueTokenMessage struct {
	ChainID         uint64
	Nonce           uint64
	Symbol          string
	Name            string
	Decimals        uint8
	MasterAuthority common.Address
}

func TestIssueToken(t *testing.T) {
	msg := &IssueTokenMessage{
		ChainID:         1212101,
		Decimals:        6,
		MasterAuthority: common.HexToAddress("0x1DFa71eC8284F0F835EDbfaEA458d38bCff446d6"),
		Name:            "USDF Stablecoin",
		Nonce:           31,
		Symbol:          "USDF",
	}

	privateKey := "76700ba1cb72480053d43b6202a16e9acbfb318b0321cfac4e55d38747bf9057"
	signature, err := sign.Message(msg, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

	req := &IssueTokenRequest{
		ChainID:         1212101,
		Decimals:        6,
		MasterAuthority: "0x1DFa71eC8284F0F835EDbfaEA458d38bCff446d6",
		Name:            "USDF Stablecoin",
		Nonce:           31,
		Symbol:          "USDF",
		Signature: Signature{
			R: signature.R,
			S: signature.S,
			V: int(signature.V),
		},
	}

	result, err := IssueToken(req)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}

	if result.Hash[:2] != "0x" {
		t.Error("Expected Hash to start with 0x")
	}

	if result.Token == "" {
		t.Error("Expected Token to be present")
	}

	if result.Token[:2] != "0x" {
		t.Error("Expected Token to start with 0x")
	}

	t.Logf("Successfully issued token: %s", result.Token)
	t.Logf("Transaction hash: %s", result.Hash)
}

func TestGetTokenInfo(t *testing.T) {
	tokenAddress := "0x57a7d1514bae23bfc4e03dbb839e1ae2a2f18192"
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

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}

	if result.Hash[:2] != "0x" {
		t.Error("Expected Hash to start with 0x")
	}

	t.Log("\nMetadata Update Result:")
	t.Log("=====================")
	t.Logf("Transaction Hash: %s", result.Hash)
}

func TestGrantAuthority(t *testing.T) {
	payload := TokenAuthorityPayload{
		ChainID:          1212101,
		Nonce:            0,
		Action:           AuthorityActionGrant,
		AuthorityType:    AuthorityTypeMasterMint,
		AuthorityAddress: common.HexToAddress("0x1DFa71eC8284F0F835EDbfaEA458d38bCff446d6"),
		Token:            common.HexToAddress("0x6ADE9688A44D058fF181Ed64ddFAFbBE5CC742Ac"),
		Value:            big.NewInt(0),
	}

	privateKey := "b1c49ed15a19a21541cd71a0837c75194756cbe81ac13c14e31213d766e84e7a"
	signature, err := sign.Message(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate signature: %v", err)
	}

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

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}

	if result.Hash[:2] != "0x" {
		t.Error("Expected Hash to start with 0x")
	}

	t.Log("\nGrant Authority Result:")
	t.Log("=====================")
	t.Logf("Action:            %s", req.Action)
	t.Logf("Authority Type:    %s", req.AuthorityType)
	t.Logf("Authority Address: %s", req.AuthorityAddress)
	t.Logf("Token:             %s", req.Token)
	t.Logf("Value:             %s", req.Value)
	t.Logf("Transaction Hash:  %s", result.Hash)
}
