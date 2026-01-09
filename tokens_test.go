package onemoney

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestTokenPayloadJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		target   func() interface{}
		validate func(*testing.T, interface{})
	}{
		{
			name: "TokenIssuePayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 9,
                "symbol": "TEST",
                "name": "Test Token",
                "decimals": 6,
                "master_authority": "0x5555555555555555555555555555555555555555",
                "is_private": true
            }`,
			target: func() interface{} { return new(TokenIssuePayload) },
			validate: func(t *testing.T, v interface{}) {
				a := assert.New(t)
				payload := v.(*TokenIssuePayload)
				a.Equal(uint64(1212101), payload.ChainID)
				a.Equal("TEST", payload.Symbol)
				a.Equal(tokenAddr("0x5555555555555555555555555555555555555555"), payload.MasterAuthority)
				a.True(payload.IsPrivate)
			},
		},
		{
			name: "UpdateMetadataPayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 10,
                "name": "Updated Token",
                "uri": "ipfs://example",
                "token": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "additional_metadata": [
                    {"key": "color", "value": "blue"}
                ]
            }`,
			target: func() interface{} { return new(UpdateMetadataPayload) },
			validate: func(t *testing.T, v interface{}) {
				a := assert.New(t)
				payload := v.(*UpdateMetadataPayload)
				a.Equal("Updated Token", payload.Name)
				a.Equal(tokenAddr("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), payload.Token)
				a.Equal(1, len(payload.AdditionalMetadata))
				a.Equal("color", payload.AdditionalMetadata[0].Key)
			},
		},
		{
			name: "TokenAuthorityPayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 11,
                "action": "Grant",
                "authority_type": "MintBurnTokens",
                "authority_address": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "token": "0xcccccccccccccccccccccccccccccccccccccccc",
                "value": 1000
            }`,
			target: func() interface{} { return new(TokenAuthorityPayload) },
			validate: func(t *testing.T, v interface{}) {
				payload := v.(*TokenAuthorityPayload)
				assertBigIntEqual(t, "1000", payload.Value)
				assert.Equal(t, tokenAddr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), payload.AuthorityAddress)
				assert.Equal(t, tokenAddr("0xcccccccccccccccccccccccccccccccccccccccc"), payload.Token)
			},
		},
		{
			name: "TokenBridgeAndMintPayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 12,
                "recipient": "0x1111111111111111111111111111111111111111",
                "value": 5000,
                "token": "0x2222222222222222222222222222222222222222",
                "source_chain_id": 99,
                "source_tx_hash": "0xsourcetx",
                "bridge_metadata": "bridge"
            }`,
			target: func() interface{} { return new(TokenBridgeAndMintPayload) },
			validate: func(t *testing.T, v interface{}) {
				payload := v.(*TokenBridgeAndMintPayload)
				assertBigIntEqual(t, "5000", payload.Value)
				assert.Equal(t, tokenAddr("0x1111111111111111111111111111111111111111"), payload.Recipient)
				assert.Equal(t, tokenAddr("0x2222222222222222222222222222222222222222"), payload.Token)
				assert.Equal(t, uint64(99), payload.SourceChainID)
			},
		},
		{
			name: "TokenBurnAndBridgePayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 13,
                "sender": "0x1234567890123456789012345678901234567890",
                "value": 42,
                "token": "0x3333333333333333333333333333333333333333",
                "destination_chain_id": 137,
                "destination_address": "dest",
                "escrow_fee": 3,
                "bridge_metadata": "meta",
                "bridge_param": "0xdeadbeef"
            }`,
			target: func() interface{} { return new(TokenBurnAndBridgePayload) },
			validate: func(t *testing.T, v interface{}) {
				payload := v.(*TokenBurnAndBridgePayload)
				assertBigIntEqual(t, "42", payload.Value)
				assertBigIntEqual(t, "3", payload.EscrowFee)
				assert.Equal(t, tokenAddr("0x1234567890123456789012345678901234567890"), payload.Sender)
				assert.Equal(t, tokenAddr("0x3333333333333333333333333333333333333333"), payload.Token)
				assert.Equal(t, hexBytesFromString(t, "0xdeadbeef"), payload.BridgeParam)
			},
		},
		{
			name: "TokenManageListPayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 14,
                "action": "Add",
                "address": "0x4444444444444444444444444444444444444444",
                "token": "0x5555555555555555555555555555555555555555"
            }`,
			target: func() interface{} { return new(TokenManageListPayload) },
			validate: func(t *testing.T, v interface{}) {
				payload := v.(*TokenManageListPayload)
				assert.Equal(t, ManageListActionAdd, payload.Action)
				assert.Equal(t, tokenAddr("0x4444444444444444444444444444444444444444"), payload.Address)
				assert.Equal(t, tokenAddr("0x5555555555555555555555555555555555555555"), payload.Token)
			},
		},
		{
			name: "PauseTokenPayload",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 15,
                "action": "Pause",
                "token": "0x6666666666666666666666666666666666666666"
            }`,
			target: func() interface{} { return new(PauseTokenPayload) },
			validate: func(t *testing.T, v interface{}) {
				payload := v.(*PauseTokenPayload)
				assert.Equal(t, PauseActionType("Pause"), payload.Action)
				assert.Equal(t, tokenAddr("0x6666666666666666666666666666666666666666"), payload.Token)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			target := tc.target()
			a := assert.New(t)
			if !a.NoError(json.Unmarshal([]byte(tc.jsonData), target)) {
				return
			}
			tc.validate(t, target)
			encoded, err := json.Marshal(target)
			if !a.NoError(err) {
				return
			}
			assertJSONEqual(t, tc.jsonData, encoded)
		})
	}
}

func TestTokenModelJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		target   func() interface{}
		validate func(*testing.T, interface{})
	}{
		{
			name: "IssueTokenRequest",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 16,
                "symbol": "REQ",
                "name": "Request Token",
                "decimals": 8,
                "master_authority": "0x7777777777777777777777777777777777777777",
                "is_private": false,
                "signature": {"r": "0x1", "s": "0x2", "v": 1}
            }`,
			target: func() interface{} { return new(IssueTokenRequest) },
			validate: func(t *testing.T, v interface{}) {
				req := v.(*IssueTokenRequest)
				assert.Equal(t, "REQ", req.Symbol)
				assert.Equal(t, uint64(16), req.Nonce)
				assert.Equal(t, tokenAddr("0x7777777777777777777777777777777777777777"), req.MasterAuthority)
				assert.Equal(t, uint64(1), req.Signature.V)
			},
		},
		{
			name: "TokenAuthorityRequest",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 17,
                "action": "Grant",
                "authority_type": "Bridge",
                "authority_address": "0x8888888888888888888888888888888888888888",
                "token": "0x9999999999999999999999999999999999999999",
                "value": 256,
                "signature": {"r": "0x3", "s": "0x4", "v": 27}
            }`,
			target: func() interface{} { return new(TokenAuthorityRequest) },
			validate: func(t *testing.T, v interface{}) {
				req := v.(*TokenAuthorityRequest)
				assertBigIntEqual(t, "256", req.Value)
				assert.Equal(t, tokenAddr("0x8888888888888888888888888888888888888888"), req.AuthorityAddress)
				assert.Equal(t, tokenAddr("0x9999999999999999999999999999999999999999"), req.Token)
				assert.Equal(t, uint64(27), req.Signature.V)
			},
		},
		{
			name: "MintTokenRequest",
			jsonData: `{
                "chain_id": 1212101,
                "nonce": 18,
                "recipient": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "value": 100,
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "signature": {"r": "0x5", "s": "0x6", "v": 28}
            }`,
			target: func() interface{} { return new(MintTokenRequest) },
			validate: func(t *testing.T, v interface{}) {
				req := v.(*MintTokenRequest)
				assertBigIntEqual(t, "100", req.Value)
				assert.Equal(t, tokenAddr("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), req.Recipient)
				assert.Equal(t, tokenAddr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), req.Token)
				assert.Equal(t, uint64(28), req.Signature.V)
			},
		},
		{
			name: "TokenInfoResponse",
			jsonData: `{
                "symbol": "INFO",
                "master_authority": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "master_mint_burn_authority": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "mint_burn_authorities": [{"allowance": "10", "minter": "0x1"}],
                "pause_authorities": ["0x2"],
                "list_authorities": ["0x3"],
                "black_list": ["0x4"],
                "white_list": ["0x5"],
                "metadata_update_authorities": ["0x6"],
                "bridge_mint_authorities": ["0x7"],
                "supply": "9999",
                "decimals": 4,
                "is_paused": false,
                "is_private": true,
                "meta": {
                    "name": "Info Token",
                    "uri": "ipfs://info",
                    "additional_metadata": [{"key": "env", "value": "test"}]
                }
            }`,
			target: func() interface{} { return new(TokenInfoResponse) },
			validate: func(t *testing.T, v interface{}) {
				resp := v.(*TokenInfoResponse)
				assert.Equal(t, "INFO", resp.Symbol)
				assert.Equal(t, "9999", resp.Supply)
				if assert.Len(t, resp.Meta.AdditionalMetadata, 1) {
					assert.Equal(t, "env", resp.Meta.AdditionalMetadata[0].Key)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			target := tc.target()
			a := assert.New(t)
			if !a.NoError(json.Unmarshal([]byte(tc.jsonData), target)) {
				return
			}
			tc.validate(t, target)
			encoded, err := json.Marshal(target)
			if !a.NoError(err) {
				return
			}
			assertJSONEqual(t, tc.jsonData, encoded)
		})
	}
}

func TestTokenBridgeAndMintPayloadSignatureHash(t *testing.T) {
	value, ok := new(big.Int).SetString("0xde0b6b3a7640000", 0)
	if !assert.True(t, ok) {
		return
	}

	payload := TokenBridgeAndMintPayload{
		ChainID:        1212101,
		Nonce:          5,
		Recipient:      tokenAddr("0x742d35cc6634c0532925a3b8d91d6f4a81b8cbc0"),
		Value:          value,
		Token:          tokenAddr("0x1234567890abcdef1234567890abcdef12345678"),
		SourceChainID:  1,
		SourceTxHash:   "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		BridgeMetadata: "",
	}

	hash, err := HashMessage(payload)
	assert.Nil(t, err)
	assert.Equal(t, "0x872fa69f0c491b401fdef9e945db6896117fe43d09aa750b261813ac7b93ef10", common.BytesToHash(hash).Hex())
}

func TestTokenBurnAndBridgePayloadSignatureHash(t *testing.T) {
	value, ok := new(big.Int).SetString("0x1dcd6500", 0)
	if !assert.True(t, ok) {
		return
	}
	escrowFee, ok := new(big.Int).SetString("0xf4240", 0)
	if !assert.True(t, ok) {
		return
	}

	payload := TokenBurnAndBridgePayload{
		ChainID:            1212101,
		Nonce:              5,
		Sender:             tokenAddr("0x742d35cc6634c0532925a3b8d91d6f4a81b8cbc0"),
		Value:              value,
		Token:              tokenAddr("0x1234567890abcdef1234567890abcdef12345678"),
		DestinationChainID: 1,
		DestinationAddress: "0x1234567890abcdef1234567890abcdef12345678",
		EscrowFee:          escrowFee,
		BridgeMetadata:     "",
		BridgeParam:        hexBytesFromString(t, "0x"),
	}

	hash, err := HashMessage(payload)
	assert.Nil(t, err)
	assert.Equal(t, "0xa48e2a8591f8bfa35ef26dd41d4ccb0a2f9550e3f6f06cd9e958cf91c89afefa", common.BytesToHash(hash).Hex())
}

func assertJSONEqual(t *testing.T, expected string, actual []byte) {
	t.Helper()
	var expObj interface{}
	var actObj interface{}
	a := assert.New(t)
	if !a.NoError(json.Unmarshal([]byte(expected), &expObj)) {
		return
	}
	if !a.NoError(json.Unmarshal(actual, &actObj)) {
		return
	}
	a.Equal(expObj, actObj)
}

func tokenAddr(hex string) common.Address {
	return common.HexToAddress(hex)
}

func assertBigIntEqual(t *testing.T, expected string, actual *big.Int) {
	t.Helper()
	a := assert.New(t)
	if !a.NotNil(actual) {
		return
	}
	exp, ok := new(big.Int).SetString(expected, 10)
	if !a.True(ok, "invalid big.Int string") {
		return
	}
	a.Zero(exp.Cmp(actual))
}

func hexBytesFromString(t *testing.T, value string) HexBytes {
	t.Helper()
	value = strings.TrimPrefix(value, "0x")
	if value == "" {
		return HexBytes{}
	}
	bytes, err := hex.DecodeString(value)
	if !assert.NoError(t, err) {
		return nil
	}
	return HexBytes(bytes)
}
