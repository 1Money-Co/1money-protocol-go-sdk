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
				if assert.NotNil(t, payload.Token) {
					assert.Equal(t, addr("0x2222222222222222222222222222222222222222"), *payload.Token)
				}
				assert.Equal(t, "12345", payload.Value)
			},
		},
		{
			name:   "BatchPayment",
			txType: TransactionTypeBatchPayment,
			data: `{
                "token": "0x1111111111111111111111111111111111111111",
                "max_fee": "5000",
                "operations": [
                    {"recipient": "0x2222222222222222222222222222222222222222", "amount": "1000"}
                ],
                "operations_hash": "0x3333333333333333333333333333333333333333333333333333333333333333",
                "batch_id": "payroll-1",
                "created_at": 1747785600
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsBatchPaymentData()
				if !assert.True(t, ok) {
					return
				}
				if assert.NotNil(t, payload.Token) {
					assert.Equal(t, addr("0x1111111111111111111111111111111111111111"), *payload.Token)
				}
				assert.Equal(t, "5000", payload.MaxFee)
				assert.Equal(t, uint64(1747785600), payload.CreatedAt)
				if assert.Len(t, payload.Operations, 1) {
					assert.Equal(t, addr("0x2222222222222222222222222222222222222222"), payload.Operations[0].Recipient)
					assert.Equal(t, "1000", payload.Operations[0].Amount)
				}
				if assert.NotNil(t, payload.OperationsHash) {
					assert.Equal(t, "0x3333333333333333333333333333333333333333333333333333333333333333", *payload.OperationsHash)
				}
				if assert.NotNil(t, payload.BatchID) {
					assert.Equal(t, "payroll-1", *payload.BatchID)
				}
			},
		},
		{
			name:   "TokenClawback",
			txType: TransactionTypeTokenClawback,
			data: `{
                "from": "0x1111111111111111111111111111111111111111",
                "recipient": "0x2222222222222222222222222222222222222222",
                "value": "777",
                "token": "0x3333333333333333333333333333333333333333"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenClawbackData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, addr("0x1111111111111111111111111111111111111111"), payload.From)
				assert.Equal(t, addr("0x2222222222222222222222222222222222222222"), payload.Recipient)
				assert.Equal(t, "777", payload.Value)
				assert.Equal(t, addr("0x3333333333333333333333333333333333333333"), payload.Token)
			},
		},
		{
			name:   "CreateMultiSig",
			txType: TransactionTypeCreateMultiSig,
			data: `{
                "signers": [
                    {"public_key": "0x02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5", "weight": 1}
                ],
                "threshold": 2,
                "multisig_address": "0x4444444444444444444444444444444444444444"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsCreateMultiSigData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, uint16(2), payload.Threshold)
				assert.Equal(t, addr("0x4444444444444444444444444444444444444444"), payload.MultisigAddress)
				if assert.Len(t, payload.Signers, 1) {
					assert.Equal(t, "0x02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5", payload.Signers[0].PublicKey)
					assert.Equal(t, uint8(1), payload.Signers[0].Weight)
				}
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
				if assert.NotNil(t, payload.Value) {
					assert.Equal(t, "1000", *payload.Value)
				}
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
                "bridge_metadata": "burn-bridge",
                "bridge_param": "0x1234abcd"
            }`,
			assert: func(t *testing.T, tx *Transaction) {
				payload, ok := tx.AsTokenBurnAndBridgeData()
				if !assert.True(t, ok) {
					return
				}
				assert.Equal(t, uint64(42161), payload.DestinationChainID)
				assert.Equal(t, "burn-bridge", payload.BridgeMetadata)
				assert.Equal(t, "0x1234abcd", payload.BridgeParam)
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
			name:   "Empty",
			txType: TransactionTypeEmpty,
			data:   `{}`,
			assert: func(t *testing.T, tx *Transaction) {
				_, ok := tx.AsEmptyData()
				assert.True(t, ok)
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

// TestBatchPayment_Unmarshal_NullOptionals proves that null optional fields
// (token, operations_hash, batch_id) decode to nil rather than failing the whole
// Transaction unmarshal — the reason those fields are pointers.
func TestBatchPayment_Unmarshal_NullOptionals(t *testing.T) {
	raw := `{"transaction_type":"BatchPayment","data":{"token":null,"max_fee":"0","operations":[],"operations_hash":null,"batch_id":null,"created_at":0}}`
	var tx Transaction
	if !assert.NoError(t, json.Unmarshal([]byte(raw), &tx)) {
		return
	}
	payload, ok := tx.AsBatchPaymentData()
	if !assert.True(t, ok) {
		return
	}
	assert.Nil(t, payload.Token)
	assert.Nil(t, payload.OperationsHash)
	assert.Nil(t, payload.BatchID)
	assert.Empty(t, payload.Operations)
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

// TestTransaction_UnmarshalClearsStaleAuthorization verifies that decoding into
// a reused Transaction value never retains a stale authorization: the
// discriminator SignatureType must select exactly one of Signature /
// MultiSignature (or neither when there is no signature).
func TestTransaction_UnmarshalClearsStaleAuthorization(t *testing.T) {
	const base = `"hash":"0x1","from":"0x1111111111111111111111111111111111111111","chain_id":1,"nonce":1,"transaction_type":"TokenMint","data":{"recipient":"0x2222222222222222222222222222222222222222","token":"0x3333333333333333333333333333333333333333","value":"1"}`
	single := `{` + base + `,"signature_type":"Single","signature":{"r":"0x1","s":"0x2","v":0}}`
	multi := `{` + base + `,"signature_type":"Multi","signature":{"account":"0x4444444444444444444444444444444444444444","signatures":[{"signer_pubkey":"0x02","signature":{"r":"0x1","s":"0x2","v":1}}]}}`
	noSig := `{` + base + `}`

	var tx Transaction // reused across decodes on purpose

	if !assert.NoError(t, json.Unmarshal([]byte(single), &tx)) {
		return
	}
	assert.NotNil(t, tx.Signature)

	// Single -> Multi: the single Signature must be cleared.
	if !assert.NoError(t, json.Unmarshal([]byte(multi), &tx)) {
		return
	}
	assert.Nil(t, tx.Signature, "stale single signature not cleared after decoding a multi signature")
	assert.NotNil(t, tx.MultiSignature)

	// Multi -> Single: the MultiSignature must be cleared.
	if !assert.NoError(t, json.Unmarshal([]byte(single), &tx)) {
		return
	}
	assert.Nil(t, tx.MultiSignature, "stale multi signature not cleared after decoding a single signature")
	assert.NotNil(t, tx.Signature)

	// Single -> no signature: both must be cleared.
	if !assert.NoError(t, json.Unmarshal([]byte(noSig), &tx)) {
		return
	}
	assert.Nil(t, tx.Signature, "stale signature not cleared when the new tx has no signature")
	assert.Nil(t, tx.MultiSignature, "stale multi signature not cleared when the new tx has no signature")
}

func TestTransaction_MarshalRoundTrip(t *testing.T) {
	// A decoded transaction must re-marshal with its payload (data) and
	// authorization (signature_type + signature) intact, so downstream consumers
	// that re-serialize it (e.g. a checkpoint's full transactions) can still
	// verify it.
	t.Run("single", func(t *testing.T) {
		input := `{"hash":"0xabc","from":"0x1111111111111111111111111111111111111111","chain_id":1212101,"nonce":1,"transaction_type":"TokenTransfer","data":{"recipient":"0x2222222222222222222222222222222222222222","token":"0x3333333333333333333333333333333333333333","value":"12345"},"signature_type":"Single","signature":{"r":"0x1","s":"0x2","v":0}}`
		var tx Transaction
		if !assert.NoError(t, json.Unmarshal([]byte(input), &tx)) {
			return
		}
		encoded, err := json.Marshal(tx)
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, string(encoded), `"signature"`)
		assert.Contains(t, string(encoded), `"data"`)

		var re Transaction
		if !assert.NoError(t, json.Unmarshal(encoded, &re)) {
			return
		}
		assert.Equal(t, "Single", re.SignatureType)
		if assert.NotNil(t, re.Signature) {
			assert.Equal(t, "0x1", re.Signature.R)
		}
		if payload, ok := re.AsTokenTransferData(); assert.True(t, ok) {
			assert.Equal(t, "12345", payload.Value)
		}
	})

	t.Run("multi", func(t *testing.T) {
		input := `{"hash":"0xabc","from":"0x1111111111111111111111111111111111111111","chain_id":1212101,"nonce":1,"transaction_type":"TokenMint","data":{"recipient":"0x2222222222222222222222222222222222222222","token":"0x3333333333333333333333333333333333333333","value":"10"},"signature_type":"Multi","signature":{"account":"0x4444444444444444444444444444444444444444","signatures":[{"signer_pubkey":"0x02","signature":{"r":"0x1","s":"0x2","v":1}}]}}`
		var tx Transaction
		if !assert.NoError(t, json.Unmarshal([]byte(input), &tx)) {
			return
		}
		encoded, err := json.Marshal(tx)
		if !assert.NoError(t, err) {
			return
		}
		var re Transaction
		if !assert.NoError(t, json.Unmarshal(encoded, &re)) {
			return
		}
		assert.Equal(t, "Multi", re.SignatureType)
		assert.Nil(t, re.Signature)
		if assert.NotNil(t, re.MultiSignature) {
			assert.Equal(t, addr("0x4444444444444444444444444444444444444444"), re.MultiSignature.Account)
			assert.Len(t, re.MultiSignature.Signatures, 1)
		}
	})
}

func TestTransaction_SignatureDecode(t *testing.T) {
	t.Run("Single", func(t *testing.T) {
		jsonData := `{
			"hash": "0xabc",
			"from": "0x1111111111111111111111111111111111111111",
			"chain_id": 1212101,
			"nonce": 1,
			"transaction_type": "TokenMint",
			"data": {"recipient": "0x2222222222222222222222222222222222222222", "token": "0x3333333333333333333333333333333333333333", "value": "10"},
			"signature_type": "Single",
			"signature": {"r": "0x1", "s": "0x2", "v": 0},
			"signature_scheme": "domain_separated",
			"memo": {"type": "purpose/SALA", "format": "text/plain", "data": "hi"}
		}`
		var tx Transaction
		if !assert.NoError(t, json.Unmarshal([]byte(jsonData), &tx)) {
			return
		}
		assert.Equal(t, "Single", tx.SignatureType)
		assert.Nil(t, tx.MultiSignature)
		if assert.NotNil(t, tx.Signature) {
			assert.Equal(t, "0x1", tx.Signature.R)
			assert.Equal(t, uint64(0), tx.Signature.V)
		}
		assert.Equal(t, SignatureSchemeDomainSeparated, tx.SignatureScheme)
		if assert.NotNil(t, tx.Memo) {
			assert.Equal(t, "hi", tx.Memo.Data)
		}
	})

	t.Run("Multi", func(t *testing.T) {
		jsonData := `{
			"hash": "0xabc",
			"from": "0x1111111111111111111111111111111111111111",
			"chain_id": 1212101,
			"nonce": 1,
			"transaction_type": "TokenMint",
			"data": {"recipient": "0x2222222222222222222222222222222222222222", "token": "0x3333333333333333333333333333333333333333", "value": "10"},
			"signature_type": "Multi",
			"signature": {
				"account": "0x4444444444444444444444444444444444444444",
				"signatures": [
					{"signer_pubkey": "0x02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5", "signature": {"r": "0x1", "s": "0x2", "v": 1}}
				]
			}
		}`
		var tx Transaction
		if !assert.NoError(t, json.Unmarshal([]byte(jsonData), &tx)) {
			return
		}
		assert.Equal(t, "Multi", tx.SignatureType)
		assert.Nil(t, tx.Signature)
		if assert.NotNil(t, tx.MultiSignature) {
			assert.Equal(t, addr("0x4444444444444444444444444444444444444444"), tx.MultiSignature.Account)
			if assert.Len(t, tx.MultiSignature.Signatures, 1) {
				assert.Equal(t, "0x02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5", tx.MultiSignature.Signatures[0].SignerPubkey)
				assert.Equal(t, uint64(1), tx.MultiSignature.Signatures[0].Signature.V)
			}
		}
	})
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

	if assert.NotNil(t, receipt.CheckpointHash) {
		assert.Equal(t, "0xabc", *receipt.CheckpointHash)
	}
	if assert.NotNil(t, receipt.CheckpointNumber) {
		assert.Equal(t, uint64(42), *receipt.CheckpointNumber)
	}
	assert.Equal(t, "0xdeadbeef", receipt.TransactionHash)
	if assert.NotNil(t, receipt.TransactionIndex) {
		assert.Equal(t, uint64(7), *receipt.TransactionIndex)
	}
	assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), receipt.From)
	assert.True(t, receipt.Success)
	assert.Equal(t, "100", receipt.FeeUsed)
	assert.Nil(t, receipt.Recipient)
	if assert.NotNil(t, receipt.TokenAddress) {
		assert.Equal(t, common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), *receipt.TokenAddress)
	}
}

// TestTransactionReceiptResponse_BatchAndEvents covers the batch-payment receipt
// detail added to match the node: batch_info, success_info and execution_events.
func TestTransactionReceiptResponse_BatchAndEvents(t *testing.T) {
	jsonData := `{
		"success": true,
		"transaction_hash": "0xdeadbeef",
		"transaction_index": 3,
		"checkpoint_hash": "0xabc",
		"checkpoint_number": 42,
		"from": "0x1234567890123456789012345678901234567890",
		"fee_used": "100",
		"recipient": null,
		"token_address": null,
		"success_info": {
			"sender": "0x1111111111111111111111111111111111111111",
			"receiver": "0x2222222222222222222222222222222222222222",
			"is_private": false,
			"message": "ok",
			"bridge_info": null
		},
		"batch_info": {
			"batch_id": "payroll-1",
			"operations_hash": "0x3333333333333333333333333333333333333333333333333333333333333333",
			"operations_count": 2,
			"total_amount": "3000",
			"failure": null
		},
		"execution_events": [
			{"event_type": "BatchStarted", "batch_id": "payroll-1", "operations_count": 2, "total_amount": "3000"},
			{"event_type": "PaymentExecuted", "operation_index": 0, "recipient": "0x2222222222222222222222222222222222222222", "amount": "1000"}
		]
	}`

	receipt := new(TransactionReceiptResponse)
	if err := json.Unmarshal([]byte(jsonData), receipt); !assert.NoError(t, err) {
		return
	}

	if assert.NotNil(t, receipt.SuccessInfo) {
		assert.Equal(t, common.HexToAddress("0x1111111111111111111111111111111111111111"), receipt.SuccessInfo.Sender)
		assert.Equal(t, "ok", receipt.SuccessInfo.Message)
		assert.Nil(t, receipt.SuccessInfo.BridgeInfo)
	}
	if assert.NotNil(t, receipt.BatchInfo) {
		if assert.NotNil(t, receipt.BatchInfo.BatchID) {
			assert.Equal(t, "payroll-1", *receipt.BatchInfo.BatchID)
		}
		assert.Equal(t, uint64(2), receipt.BatchInfo.OperationsCount)
		assert.Equal(t, "3000", receipt.BatchInfo.TotalAmount)
		if assert.NotNil(t, receipt.BatchInfo.OperationsHash) {
			assert.Equal(t, common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"), *receipt.BatchInfo.OperationsHash)
		}
	}
	if assert.Len(t, receipt.ExecutionEvents, 2) {
		assert.Equal(t, "BatchStarted", receipt.ExecutionEvents[0].EventType)
		assert.Equal(t, "PaymentExecuted", receipt.ExecutionEvents[1].EventType)
		if assert.NotNil(t, receipt.ExecutionEvents[1].OperationIndex) {
			assert.Equal(t, uint64(0), *receipt.ExecutionEvents[1].OperationIndex)
		}
		if assert.NotNil(t, receipt.ExecutionEvents[1].Amount) {
			assert.Equal(t, "1000", *receipt.ExecutionEvents[1].Amount)
		}
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
		"counter_signature": {
			"signer_bitmask": "0x0f",
			"signature": "0xaggregate",
			"validator_public_keys": ["0x01", "0x02"]
		},
		"fee": "100",
		"fee_bound": true
	}`

	finalized := new(FinalizedTransactionResponse)
	if err := json.Unmarshal([]byte(jsonData), finalized); !assert.NoError(t, err) {
		return
	}

	if assert.NotNil(t, finalized.CheckpointHash) {
		assert.Equal(t, "0xabc", *finalized.CheckpointHash)
	}
	if assert.NotNil(t, finalized.CheckpointNumber) {
		assert.Equal(t, uint64(42), *finalized.CheckpointNumber)
	}
	assert.Equal(t, "0xdeadbeef", finalized.TransactionHash)
	if assert.NotNil(t, finalized.TransactionIndex) {
		assert.Equal(t, uint64(7), *finalized.TransactionIndex)
	}
	assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), finalized.From)
	assert.True(t, finalized.Success)
	assert.Equal(t, "100", finalized.FeeUsed)
	assert.Nil(t, finalized.TokenAddress)
	if assert.NotNil(t, finalized.Recipient) {
		assert.Equal(t, common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), *finalized.Recipient)
	}
	assert.Equal(t, uint64(99), finalized.Epoch)
	assert.Equal(t, "0x0f", finalized.CounterSignature.SignerBitmask)
	assert.Equal(t, "0xaggregate", finalized.CounterSignature.Signature)
	assert.Equal(t, []string{"0x01", "0x02"}, finalized.CounterSignature.ValidatorPublicKeys)
	if assert.NotNil(t, finalized.Fee) {
		assert.Equal(t, "100", *finalized.Fee)
	}
	assert.True(t, finalized.FeeBound)
}
