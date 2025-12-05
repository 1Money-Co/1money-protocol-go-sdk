package onemoney

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestCheckpointTransactions_JSON(t *testing.T) {
	t.Run("hashes", func(t *testing.T) {
		input := `{"transactions": ["0x1", "0x2"]}`
		target := struct {
			Transactions CheckpointTransactions `json:"transactions"`
		}{}
		require := assert.New(t)
		if !require.NoError(json.Unmarshal([]byte(input), &target)) {
			return
		}
		require.Nil(target.Transactions.Full)
		require.Equal([]string{"0x1", "0x2"}, target.Transactions.Hashes)

		encoded, err := json.Marshal(target)
		require.NoError(err)
		require.JSONEq(input, string(encoded))
	})

	t.Run("full", func(t *testing.T) {
		input := `{"transactions": [{"hash": "0xabc", "checkpoint_hash": "0xparent", "checkpoint_number": 1, "from": "0x1111111111111111111111111111111111111111", "transaction_type": "TokenMint", "chain_id": 1212101, "nonce": 1, "signature": null, "transaction_index": 0}]}`
		target := struct {
			Transactions CheckpointTransactions `json:"transactions"`
		}{}
		require := assert.New(t)
		if !require.NoError(json.Unmarshal([]byte(input), &target)) {
			return
		}
		require.Nil(target.Transactions.Hashes)
		require.Len(target.Transactions.Full, 1)
		require.Equal("0xabc", target.Transactions.Full[0].Hash)

		encoded, err := json.Marshal(target)
		require.NoError(err)
		require.JSONEq(input, string(encoded))
	})

	t.Run("invalid", func(t *testing.T) {
		input := `{"transactions": 123}`
		target := struct {
			Transactions CheckpointTransactions `json:"transactions"`
		}{}
		assert.Error(t, json.Unmarshal([]byte(input), &target))
	})
}

func TestCheckpointJSONRoundTrip(t *testing.T) {
	original := Checkpoint{
		ExtraData:        "extra",
		Hash:             "0xhash",
		Number:           42,
		ParentHash:       "0xparent",
		ReceiptsRoot:     "0xreceipts",
		StateRoot:        "0xstate",
		Timestamp:        123456,
		TransactionsRoot: "0xtrx",
		Transactions: CheckpointTransactions{
			Hashes: []string{"0x1", "0x2"},
		},
		Size: 2,
	}

	data, err := json.Marshal(original)
	if !assert.NoError(t, err) {
		return
	}

	var decoded Checkpoint
	if !assert.NoError(t, json.Unmarshal(data, &decoded)) {
		return
	}

	assert.Equal(t, original, decoded)

	// Full transactions
	full := Checkpoint{
		ExtraData:        "extra",
		Hash:             "0xhash",
		Number:           43,
		ParentHash:       "0xparent",
		ReceiptsRoot:     "0xreceipts",
		StateRoot:        "0xstate",
		Timestamp:        654321,
		TransactionsRoot: "0xtrx",
		Transactions: CheckpointTransactions{
			Full: []Transaction{
				{
					Hash:             "0xabc",
					CheckpointHash:   "0xparent",
					CheckpointNumber: 43,
					From:             common.HexToAddress("0x1111111111111111111111111111111111111111"),
					TransactionType:  TransactionTypeTokenMint,
					ChainID:          1212101,
					Nonce:            1,
					TransactionIndex: 0,
				},
			},
		},
		Size: 1,
	}

	dataFull, err := json.Marshal(full)
	if !assert.NoError(t, err) {
		return
	}

	var decodedFull Checkpoint
	if !assert.NoError(t, json.Unmarshal(dataFull, &decodedFull)) {
		return
	}

	assert.Equal(t, full.Hash, decodedFull.Hash)
	assert.Len(t, decodedFull.Transactions.Full, len(full.Transactions.Full))
	for i := range full.Transactions.Full {
		assert.Equal(t, full.Transactions.Full[i].Hash, decodedFull.Transactions.Full[i].Hash)
		assert.Equal(t, full.Transactions.Full[i].TransactionType, decodedFull.Transactions.Full[i].TransactionType)
	}
}

func TestCheckpointNumberJSON(t *testing.T) {
	jsonData := `{"number": 99}`
	var num CheckpointNumber
	if !assert.NoError(t, json.Unmarshal([]byte(jsonData), &num)) {
		return
	}

	assert.Equal(t, uint64(99), num.Number)

	data, err := json.Marshal(num)
	if !assert.NoError(t, err) {
		return
	}

	assert.JSONEq(t, jsonData, string(data))
}
