package onemoney

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

type payloadTestCase struct {
	name   string
	txType TransactionType
	data   string
	assert func(t *testing.T, tx *Transaction)
}

func addr(hex string) common.Address {
	return common.HexToAddress(hex)
}

func payloadCases() []payloadTestCase {
	return []payloadTestCase{
		{
			name:   "TokenCreate",
			txType: TransactionTypeTokenCreate,
			data: `{
                "symbol": "TEST",
                "name": "Test Token",
                "decimals": 6,
                "master_authority": "0x5555555555555555555555555555555555555555",
                "is_private": false
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenCreateData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, "TEST", payload.Symbol)
				assert.Equal(t, "Test Token", payload.Name)
				assert.Equal(t, uint8(6), payload.Decimals)
				assert.Equal(t, addr("0x5555555555555555555555555555555555555555"), payload.MasterAuthority)
				assert.False(t, payload.IsPrivate)
			},
		},
		{
			name:   "TokenTransfer",
			txType: TransactionTypeTokenTransfer,
			data: `{
                "recipient": "0x1111111111111111111111111111111111111111",
                "token": "0x2222222222222222222222222222222222222222",
                "value": "12345"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenTransferData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0x1111111111111111111111111111111111111111"), payload.Recipient)
				assert.Equal(t, addr("0x2222222222222222222222222222222222222222"), payload.Token)
				assert.Equal(t, "12345", payload.Value)
			},
		},
		{
			name:   "TokenGrantAuthority",
			txType: TransactionTypeTokenGrantAuthority,
			data: `{
                "authority_address": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "authority_type": "MintBurnTokens",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "value": "1000"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenGrantAuthorityData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), payload.AuthorityAddress)
				assert.Equal(t, AuthorityTypeMintBurnTokens, payload.AuthorityType)
				assert.Equal(t, addr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), payload.Token)
				assert.Equal(t, "1000", payload.Value)
			},
		},
		{
			name:   "TokenRevokeAuthority",
			txType: TransactionTypeTokenRevokeAuthority,
			data: `{
                "authority_address": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "authority_type": "Pause",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "value": "0"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenRevokeAuthorityData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, AuthorityTypePause, payload.AuthorityType)
			},
		},
		{
			name:   "TokenBlacklistAccount",
			txType: TransactionTypeTokenBlacklistAccount,
			data: `{
                "address": "0x9999999999999999999999999999999999999999",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenBlacklistAccountData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0x9999999999999999999999999999999999999999"), payload.Address)
			},
		},
		{
			name:   "TokenWhitelistAccount",
			txType: TransactionTypeTokenWhitelistAccount,
			data: `{
                "address": "0x8888888888888888888888888888888888888888",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenWhitelistAccountData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0x8888888888888888888888888888888888888888"), payload.Address)
			},
		},
		{
			name:   "TokenMint",
			txType: TransactionTypeTokenMint,
			data: `{
                "recipient": "0x2222222222222222222222222222222222222222",
                "value": "5000000",
                "token": "0x3333333333333333333333333333333333333333"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenMintData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, "5000000", payload.Value)
			},
		},
		{
			name:   "TokenBridgeAndMint",
			txType: TransactionTypeTokenBridgeAndMint,
			data: `{
                "recipient": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "value": "999",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "source_chain_id": 42,
                "source_tx_hash": "0xsourcetx",
                "bridge_metadata": "bridge-data"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenBridgeAndMintData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, uint64(42), payload.SourceChainID)
				assert.Equal(t, "bridge-data", payload.BridgeMetadata)
			},
		},
		{
			name:   "TokenBurn",
			txType: TransactionTypeTokenBurn,
			data: `{
                "recipient": "0x1111111111111111111111111111111111111111",
                "value": "10",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenBurnData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, "10", payload.Value)
			},
		},
		{
			name:   "TokenBurnAndBridge",
			txType: TransactionTypeTokenBurnAndBridge,
			data: `{
                "sender": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
                "value": "5",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                "destination_chain_id": 42161,
                "destination_address": "0xcccccccccccccccccccccccccccccccccccccccc",
                "escrow_fee": "1",
                "bridge_metadata": "burn-bridge"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenBurnAndBridgeData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, uint64(42161), payload.DestinationChainID)
				assert.Equal(t, "burn-bridge", payload.BridgeMetadata)
			},
		},
		{
			name:   "TokenCloseAccount",
			txType: TransactionTypeTokenCloseAccount,
			data: `{
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenCloseAccountData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), payload.Token)
			},
		},
		{
			name:   "TokenPause",
			txType: TransactionTypeTokenPause,
			data: `{
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenPauseData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), payload.Token)
			},
		},
		{
			name:   "TokenUnpause",
			txType: TransactionTypeTokenUnpause,
			data: `{
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenUnpauseData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), payload.Token)
			},
		},
		{
			name:   "TokenUpdateMetadata",
			txType: TransactionTypeTokenUpdateMetadata,
			data: `{
                "metadata": {
                    "name": "New Token",
                    "uri": "ipfs://example",
                    "additional_metadata": [
                        {"key": "color", "value": "blue"}
                    ]
                },
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenUpdateMetadataData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, "New Token", payload.Metadata.Name)
				if assert.Len(t, payload.Metadata.AdditionalMetadata, 1) {
					assert.Equal(t, "color", payload.Metadata.AdditionalMetadata[0].Key)
					assert.Equal(t, "blue", payload.Metadata.AdditionalMetadata[0].Value)
				}
			},
		},
		{
			name:   "Raw",
			txType: TransactionTypeRaw,
			data: `{
                "input": "0xdeadbeef",
                "token": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsRawTransactionData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, "0xdeadbeef", payload.Input)
				assert.Equal(t, addr("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), payload.Token)
			},
		},
	}
}

func TestTransaction_Unmarshal_AllPayloads(t *testing.T) {
	for _, tc := range payloadCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			jsonData := fmt.Sprintf(`{
                "transaction_type": "%s",
                "data": %s,
                "chain_id": 1212101,
                "checkpoint_hash": "0xcheckpoint",
                "checkpoint_number": 1,
                "from": "0x1234567890123456789012345678901234567890",
                "hash": "0xhash",
                "nonce": 1,
                "recent_checkpoint": 1,
                "transaction_index": 0
            }`, tc.txType, tc.data)

			var tx Transaction
			if assert.NoError(t, json.Unmarshal([]byte(jsonData), &tx)) {
				assert.Equal(t, tc.txType, tx.TransactionType)
				tc.assert(t, &tx)
			}
		})
	}
}

func TestTransaction_Unmarshal_UnknownType(t *testing.T) {
	jsonData := `{
        "transaction_type": "unknown_future_type",
        "data": {
            "some_field": "some_value",
            "another_field": 123
        },
        "chain_id": 1212101,
        "checkpoint_hash": "0xmnop",
        "checkpoint_number": 400,
        "from": "0x7777777777777777777777777777777777777777",
        "hash": "0xhash999",
        "nonce": 15,
        "recent_checkpoint": 400,
        "transaction_index": 2
    }`

	var tx Transaction
	if assert.NoError(t, json.Unmarshal([]byte(jsonData), &tx)) {
		assert.Equal(t, TransactionType("unknown_future_type"), tx.TransactionType)
		payload, ok := tx.Data.(UnknownTransactionPayload)
		if assert.True(t, ok) {
			assert.Equal(t, "some_value", payload["some_field"])
		}
	}
}

func TestTransaction_TypeSafeHelpers_WrongType(t *testing.T) {
	tx := Transaction{
		TransactionType: TransactionTypeTokenMint,
		Data:            &TokenMintData{},
	}

	_, ok := tx.AsTokenCreateData()
	assert.False(t, ok)
	_, ok = tx.AsTokenBurnData()
	assert.False(t, ok)
	_, ok = tx.AsTokenPauseData()
	assert.False(t, ok)
	_, ok = tx.AsTokenUnpauseData()
	assert.False(t, ok)

	_, ok = tx.AsTokenMintData()
	assert.True(t, ok)
}

func TestTransactionReceiptResponse_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"checkpoint_hash": "0xabc",
		"checkpoint_number": 42,
		"transaction_hash": "0xdeadbeef",
		"transaction_index": 7,
		"from": "0x1234567890123456789012345678901234567890",
		"success": true,
		"fee_used": "100",
		"to": null,
		"recipient": null,
		"token_address": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	}`

	receipt := new(TransactionReceiptResponse)
	if err := json.Unmarshal([]byte(jsonData), receipt); !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "0xabc", receipt.CheckpointHash)
	assert.Equal(t, uint64(42), receipt.CheckpointNumber)
	assert.Equal(t, "0xdeadbeef", receipt.TransactionHash)
	assert.Equal(t, 7, receipt.TransactionIndex)
	assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), receipt.From)
	assert.True(t, receipt.Success)
	assert.Equal(t, "100", receipt.FeeUsed)
	assert.Nil(t, receipt.To)
	assert.Nil(t, receipt.Recipient)
	if assert.NotNil(t, receipt.TokenAddress) {
		assert.Equal(t, common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), *receipt.TokenAddress)
	}
}

func TestFinalizedTransactionResponse_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"checkpoint_hash": "0xabc",
		"checkpoint_number": 42,
		"transaction_hash": "0xdeadbeef",
		"transaction_index": 7,
		"from": "0x1234567890123456789012345678901234567890",
		"success": true,
		"fee_used": "100",
		"recipient": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"epoch": 99,
		"counter_signatures": [
			{
				"r": "0x1",
				"s": "0x2",
				"v": 0
			}
		]
	}`

	finalized := new(FinalizedTransactionResponse)
	if err := json.Unmarshal([]byte(jsonData), finalized); !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "0xabc", finalized.CheckpointHash)
	assert.Equal(t, uint64(42), finalized.CheckpointNumber)
	assert.Equal(t, "0xdeadbeef", finalized.TransactionHash)
	assert.Equal(t, 7, finalized.TransactionIndex)
	assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), finalized.From)
	assert.True(t, finalized.Success)
	assert.Equal(t, "100", finalized.FeeUsed)
	assert.Nil(t, finalized.TokenAddress)
	assert.Nil(t, finalized.To)
	if assert.NotNil(t, finalized.Recipient) {
		assert.Equal(t, common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), *finalized.Recipient)
	}
	assert.Equal(t, uint64(99), finalized.Epoch)
	assert.Len(t, finalized.CounterSignatures, 1)
	assert.Equal(t, "0x1", finalized.CounterSignatures[0].R)
	assert.Equal(t, "0x2", finalized.CounterSignatures[0].S)
	assert.Equal(t, uint64(0), finalized.CounterSignatures[0].V)
}
