package onemoney

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This file gives every read/query interface a default-runnable unit test
// (route + params + response decode) using an in-memory http.RoundTripper, so
// `go test ./...` guards them without a live node or the ENABLE_HTTP_CLIENT_TESTS
// socket path. Edge cases focus on the error-prone spots: nullable
// pending-transaction fields, polymorphic signatures, the checkpoint `full`
// flag, the estimate_fee `to` param, pricing-id path escaping, and non-200 /
// malformed responses.

// newQueryClient returns a Client whose GET requests are answered in memory with
// a 200 and respBody, recording the full request URL into the returned pointer.
func newQueryClient(t *testing.T, respBody string) (*Client, *string) {
	t.Helper()
	return newQueryClientStatus(t, http.StatusOK, respBody)
}

// newQueryClientStatus is newQueryClient with an explicit status code, for
// error-path coverage.
func newQueryClientStatus(t *testing.T, status int, respBody string) (*Client, *string) {
	t.Helper()
	gotURL := new(string)
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*gotURL = r.URL.String()
		if r.Method != http.MethodGet {
			t.Errorf("HTTP method = %s, want GET", r.Method)
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})}
	return NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc)), gotURL
}

// -----------------------------------------------------------------------------
// Accounts
// -----------------------------------------------------------------------------

func TestGetAccountNonce_Query(t *testing.T) {
	address := addr("0x1111111111111111111111111111111111111111")
	c, gotURL := newQueryClient(t, `{"nonce": 0}`)

	resp, err := c.GetAccountNonce(context.Background(), address)
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, *gotURL, "/v1/accounts/nonce")
	assert.Contains(t, *gotURL, "address="+address.Hex())
	// Nonce 0 must decode as 0, not be lost.
	assert.Equal(t, uint64(0), resp.Nonce)
}

func TestGetAccountBbNonce_Query(t *testing.T) {
	address := addr("0x2222222222222222222222222222222222222222")
	c, gotURL := newQueryClient(t, `{"bbnonce": 42}`)

	resp, err := c.GetAccountBbNonce(context.Background(), address)
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, *gotURL, "/v1/accounts/bbnonce")
	assert.Contains(t, *gotURL, "address="+address.Hex())
	assert.Equal(t, uint64(42), resp.BbNonce)
}

func TestGetTokenAccount_Query(t *testing.T) {
	address := addr("0x3333333333333333333333333333333333333333")
	token := addr("0x4444444444444444444444444444444444444444")
	// Balance is a decimal U256 string on the wire; a large value must survive
	// verbatim (not be parsed into a lossy numeric type).
	c, gotURL := newQueryClient(t, `{"balance": "115792089237316195423570985008687907853269984665640564039457584007913129639935", "nonce": 7}`)

	resp, err := c.GetTokenAccount(context.Background(), address, token)
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, *gotURL, "/v1/accounts/token_account")
	assert.Contains(t, *gotURL, "address="+address.Hex())
	assert.Contains(t, *gotURL, "token="+token.Hex())
	assert.Equal(t, "115792089237316195423570985008687907853269984665640564039457584007913129639935", resp.Balance)
	assert.Equal(t, uint64(7), resp.Nonce)
}

// -----------------------------------------------------------------------------
// Transactions
// -----------------------------------------------------------------------------

func TestGetTransactionByHash_Query(t *testing.T) {
	t.Run("confirmed_single", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `{
			"hash": "0xdeadbeef",
			"checkpoint_hash": "0xcp",
			"checkpoint_number": 100,
			"transaction_index": 2,
			"from": "0x1111111111111111111111111111111111111111",
			"chain_id": 1212101,
			"nonce": 5,
			"transaction_type": "TokenTransfer",
			"data": {"recipient": "0x2222222222222222222222222222222222222222", "token": "0x3333333333333333333333333333333333333333", "value": "100"},
			"signature_type": "Single",
			"signature": {"r": "0x1", "s": "0x2", "v": 0}
		}`)

		tx, err := c.GetTransactionByHash(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/transactions/by_hash")
		assert.Contains(t, *gotURL, "hash=0xdeadbeef")
		if assert.NotNil(t, tx.CheckpointHash) {
			assert.Equal(t, "0xcp", *tx.CheckpointHash)
		}
		if assert.NotNil(t, tx.CheckpointNumber) {
			assert.Equal(t, uint64(100), *tx.CheckpointNumber)
		}
		if assert.NotNil(t, tx.TransactionIndex) {
			assert.Equal(t, uint64(2), *tx.TransactionIndex)
		}
		assert.Equal(t, "Single", tx.SignatureType)
		assert.NotNil(t, tx.Signature)
		assert.Nil(t, tx.MultiSignature)
		if payload, ok := tx.AsTokenTransferData(); assert.True(t, ok) {
			if assert.NotNil(t, payload.Token) {
				assert.Equal(t, addr("0x3333333333333333333333333333333333333333"), *payload.Token)
			}
		}
	})

	t.Run("pending_null_checkpoint_fields", func(t *testing.T) {
		// A submitted-but-not-yet-checkpointed transaction returns null for these
		// fields; they must decode to nil pointers, never to 0 / "" (which would
		// read as "checkpoint 0" / "index 0").
		c, _ := newQueryClient(t, `{
			"hash": "0xdeadbeef",
			"checkpoint_hash": null,
			"checkpoint_number": null,
			"transaction_index": null,
			"from": "0x1111111111111111111111111111111111111111",
			"chain_id": 1212101,
			"nonce": 5,
			"transaction_type": "TokenTransfer",
			"data": {"recipient": "0x2222222222222222222222222222222222222222", "token": null, "value": "100"},
			"signature_type": "Single",
			"signature": {"r": "0x1", "s": "0x2", "v": 0}
		}`)

		tx, err := c.GetTransactionByHash(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, tx.CheckpointHash)
		assert.Nil(t, tx.CheckpointNumber)
		assert.Nil(t, tx.TransactionIndex)
		// A native-value transfer reports a null token.
		if payload, ok := tx.AsTokenTransferData(); assert.True(t, ok) {
			assert.Nil(t, payload.Token)
		}
	})

	t.Run("multisig", func(t *testing.T) {
		c, _ := newQueryClient(t, `{
			"hash": "0xdeadbeef",
			"from": "0x1111111111111111111111111111111111111111",
			"chain_id": 1212101,
			"nonce": 5,
			"transaction_type": "TokenTransfer",
			"data": {"recipient": "0x2222222222222222222222222222222222222222", "token": "0x3333333333333333333333333333333333333333", "value": "100"},
			"signature_type": "Multi",
			"signature": {"account": "0x4444444444444444444444444444444444444444", "signatures": [{"signer_pubkey": "0x02", "signature": {"r": "0x1", "s": "0x2", "v": 1}}]}
		}`)

		tx, err := c.GetTransactionByHash(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "Multi", tx.SignatureType)
		assert.Nil(t, tx.Signature)
		if assert.NotNil(t, tx.MultiSignature) {
			assert.Equal(t, addr("0x4444444444444444444444444444444444444444"), tx.MultiSignature.Account)
			assert.Len(t, tx.MultiSignature.Signatures, 1)
		}
	})
}

func TestGetTransactionReceipt_Query(t *testing.T) {
	t.Run("basic_pending", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `{
			"success": true,
			"transaction_hash": "0xdeadbeef",
			"transaction_index": null,
			"checkpoint_hash": null,
			"checkpoint_number": null,
			"from": "0x1111111111111111111111111111111111111111",
			"fee_used": "100",
			"recipient": null,
			"token_address": null
		}`)

		receipt, err := c.GetTransactionReceipt(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/transactions/receipt/by_hash")
		assert.Contains(t, *gotURL, "hash=0xdeadbeef")
		assert.True(t, receipt.Success)
		assert.Equal(t, "100", receipt.FeeUsed)
		assert.Nil(t, receipt.CheckpointHash)
		assert.Nil(t, receipt.CheckpointNumber)
		assert.Nil(t, receipt.TransactionIndex)
		assert.Nil(t, receipt.Recipient)
		assert.Nil(t, receipt.TokenAddress)
	})

	t.Run("batch_detail", func(t *testing.T) {
		c, _ := newQueryClient(t, `{
			"success": true,
			"transaction_hash": "0xdeadbeef",
			"transaction_index": 0,
			"checkpoint_hash": "0xcp",
			"checkpoint_number": 5,
			"from": "0x1111111111111111111111111111111111111111",
			"fee_used": "100",
			"recipient": null,
			"token_address": "0x3333333333333333333333333333333333333333",
			"success_info": {"sender": "0x1111111111111111111111111111111111111111", "receiver": "0x2222222222222222222222222222222222222222", "is_private": false, "message": "ok", "bridge_info": null},
			"batch_info": {"batch_id": "payroll-1", "operations_hash": "0x3333333333333333333333333333333333333333333333333333333333333333", "operations_count": 2, "total_amount": "3000", "failure": null},
			"execution_events": [{"event_type": "PaymentExecuted", "operation_index": 1, "recipient": "0x2222222222222222222222222222222222222222", "amount": "1000"}]
		}`)

		receipt, err := c.GetTransactionReceipt(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		if assert.NotNil(t, receipt.BatchInfo) {
			assert.Equal(t, uint64(2), receipt.BatchInfo.OperationsCount)
			assert.Equal(t, "3000", receipt.BatchInfo.TotalAmount)
		}
		if assert.NotNil(t, receipt.SuccessInfo) {
			assert.Equal(t, "ok", receipt.SuccessInfo.Message)
		}
		if assert.Len(t, receipt.ExecutionEvents, 1) {
			assert.Equal(t, "PaymentExecuted", receipt.ExecutionEvents[0].EventType)
		}
	})
}

func TestGetFinalizedTransaction_Query(t *testing.T) {
	t.Run("with_fee_and_counter_signature", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `{
			"success": true,
			"transaction_hash": "0xdeadbeef",
			"transaction_index": 7,
			"checkpoint_hash": "0xcp",
			"checkpoint_number": 42,
			"from": "0x1111111111111111111111111111111111111111",
			"fee_used": "100",
			"recipient": "0x2222222222222222222222222222222222222222",
			"token_address": null,
			"epoch": 99,
			"counter_signature": {"signer_bitmask": "0x0f", "signature": "0xagg", "validator_public_keys": ["0x01", "0x02"]},
			"fee": "100",
			"fee_bound": true
		}`)

		fin, err := c.GetFinalizedTransaction(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/transactions/finalized/by_hash")
		assert.Contains(t, *gotURL, "hash=0xdeadbeef")
		assert.Equal(t, uint64(99), fin.Epoch)
		assert.Equal(t, "0x0f", fin.CounterSignature.SignerBitmask)
		assert.Equal(t, []string{"0x01", "0x02"}, fin.CounterSignature.ValidatorPublicKeys)
		if assert.NotNil(t, fin.Fee) {
			assert.Equal(t, "100", *fin.Fee)
		}
		assert.True(t, fin.FeeBound)
		// Flattened receipt fields remain reachable.
		assert.Equal(t, "100", fin.FeeUsed)
	})

	t.Run("fee_absent_is_nil", func(t *testing.T) {
		c, _ := newQueryClient(t, `{
			"success": true,
			"transaction_hash": "0xdeadbeef",
			"from": "0x1111111111111111111111111111111111111111",
			"fee_used": "100",
			"epoch": 99,
			"counter_signature": {"signer_bitmask": "0x0f", "signature": "0xagg", "validator_public_keys": []}
		}`)

		fin, err := c.GetFinalizedTransaction(context.Background(), "0xdeadbeef")
		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, fin.Fee)
		assert.False(t, fin.FeeBound)
	})
}

func TestGetEstimateFee_Query(t *testing.T) {
	from := addr("0x1111111111111111111111111111111111111111")
	to := addr("0x2222222222222222222222222222222222222222")
	token := addr("0x3333333333333333333333333333333333333333")

	t.Run("params_and_plan", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `{"fee": "500", "plan": "standard"}`)

		resp, err := c.GetEstimateFee(context.Background(), from, to, token, "1000")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/transactions/estimate_fee")
		assert.Contains(t, *gotURL, "from="+from.Hex())
		// `to` is required by the handler even though the OpenAPI annotation omits
		// it; the SDK must send it.
		assert.Contains(t, *gotURL, "to="+to.Hex())
		assert.Contains(t, *gotURL, "token="+token.Hex())
		assert.Contains(t, *gotURL, "value=1000")
		assert.Equal(t, "500", resp.Fee)
		if assert.NotNil(t, resp.Plan) {
			assert.Equal(t, "standard", *resp.Plan)
		}
	})

	t.Run("plan_absent_is_nil", func(t *testing.T) {
		c, _ := newQueryClient(t, `{"fee": "500"}`)

		resp, err := c.GetEstimateFee(context.Background(), from, to, token, "1000")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "500", resp.Fee)
		assert.Nil(t, resp.Plan)
	})
}

// -----------------------------------------------------------------------------
// Checkpoints
// -----------------------------------------------------------------------------

func TestGetCheckpointNumber_Query(t *testing.T) {
	c, gotURL := newQueryClient(t, `{"number": 12345}`)

	resp, err := c.GetCheckpointNumber(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, *gotURL, "/v1/checkpoints/number")
	// No query string on this endpoint.
	assert.NotContains(t, *gotURL, "?")
	assert.Equal(t, uint64(12345), resp.Number)
}

func TestGetCheckpointByHash_Query(t *testing.T) {
	t.Run("default_hashes_only", func(t *testing.T) {
		// Default: full=false, transactions arrive as a bare array of hash strings,
		// and size may be null.
		c, gotURL := newQueryClient(t, `{
			"hash": "0xcp", "number": 10, "parent_hash": "0xpar",
			"transactions_root": "0xtr", "receipts_root": "0xrr", "state_root": "0xsr",
			"timestamp": 123, "size": null,
			"transactions": ["0xa", "0xb"]
		}`)

		cp, err := c.GetCheckpointByHash(context.Background(), "0xcp")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/checkpoints/by_hash")
		assert.Contains(t, *gotURL, "hash=0xcp")
		assert.Contains(t, *gotURL, "full=false")
		assert.Equal(t, "0xcp", cp.Hash)
		assert.Nil(t, cp.Size)
		assert.Equal(t, []string{"0xa", "0xb"}, cp.Transactions.Hashes)
		assert.Nil(t, cp.Transactions.Full)
	})

	t.Run("full_transactions", func(t *testing.T) {
		// WithFullTransactions: full=true, transactions arrive as full objects.
		c, gotURL := newQueryClient(t, `{
			"hash": "0xcp", "number": 10, "parent_hash": "0xpar",
			"transactions_root": "0xtr", "receipts_root": "0xrr", "state_root": "0xsr",
			"timestamp": 123, "size": 2048,
			"transactions": [{"hash": "0xa", "from": "0x1111111111111111111111111111111111111111", "chain_id": 1212101, "nonce": 1, "transaction_type": "TokenMint", "data": {"recipient": "0x2222222222222222222222222222222222222222", "token": "0x3333333333333333333333333333333333333333", "value": "10"}, "signature_type": "Single", "signature": {"r": "0x1", "s": "0x2", "v": 0}}]
		}`)

		cp, err := c.GetCheckpointByHash(context.Background(), "0xcp", WithFullTransactions())
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "full=true")
		if assert.NotNil(t, cp.Size) {
			assert.Equal(t, uint64(2048), *cp.Size)
		}
		assert.Nil(t, cp.Transactions.Hashes)
		if assert.Len(t, cp.Transactions.Full, 1) {
			assert.Equal(t, "0xa", cp.Transactions.Full[0].Hash)
			assert.Equal(t, TransactionTypeTokenMint, cp.Transactions.Full[0].TransactionType)
		}
	})
}

func TestGetCheckpointByNumber_Query(t *testing.T) {
	// Number 0 must be sent verbatim (not omitted), and full defaults to false.
	c, gotURL := newQueryClient(t, `{
		"hash": "0xcp", "number": 0, "parent_hash": "0xpar",
		"transactions_root": "0xtr", "receipts_root": "0xrr", "state_root": "0xsr",
		"timestamp": 123, "size": 1,
		"transactions": []
	}`)

	cp, err := c.GetCheckpointByNumber(context.Background(), 0)
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, *gotURL, "/v1/checkpoints/by_number")
	assert.Contains(t, *gotURL, "number=0")
	assert.Contains(t, *gotURL, "full=false")
	assert.Equal(t, uint64(0), cp.Number)
}

func TestGetCheckpointReceiptsByNumber_Query(t *testing.T) {
	t.Run("array_with_batch_element", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `[
			{"success": true, "transaction_hash": "0xa", "transaction_index": 0, "checkpoint_hash": "0xcp", "checkpoint_number": 9, "from": "0x1111111111111111111111111111111111111111", "fee_used": "10", "recipient": null, "token_address": null, "batch_info": {"batch_id": "b1", "operations_hash": null, "operations_count": 3, "total_amount": "30", "failure": null}}
		]`)

		receipts, err := c.GetCheckpointReceiptsByNumber(context.Background(), 9)
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/checkpoints/receipts/by_number")
		assert.Contains(t, *gotURL, "number=9")
		if assert.Len(t, receipts, 1) {
			assert.Equal(t, "0xa", receipts[0].TransactionHash)
			if assert.NotNil(t, receipts[0].BatchInfo) {
				assert.Equal(t, uint64(3), receipts[0].BatchInfo.OperationsCount)
			}
		}
	})

	t.Run("empty_array", func(t *testing.T) {
		c, _ := newQueryClient(t, `[]`)

		receipts, err := c.GetCheckpointReceiptsByNumber(context.Background(), 9)
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, receipts)
	})
}

// -----------------------------------------------------------------------------
// Misc: status and pricing
// -----------------------------------------------------------------------------

func TestGetNativeWriteStatus_Query(t *testing.T) {
	t.Run("dual_activated", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `{
			"native_write_mode": "dual",
			"read_only": false,
			"activation_source": "capability_full",
			"dual_activated_at_secs": 1700000000,
			"native_domain_separated_transactions": {"support_count": 13, "required_count": 13, "full_support": true}
		}`)

		s, err := c.GetNativeWriteStatus(context.Background())
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/api/status")
		assert.Equal(t, NativeWriteModeDual, s.NativeWriteMode)
		assert.Equal(t, ActivationSourceCapabilityFull, s.ActivationSource)
		assert.False(t, s.ReadOnly)
		if assert.NotNil(t, s.DualActivatedAtSecs) {
			assert.Equal(t, uint64(1700000000), *s.DualActivatedAtSecs)
		}
		assert.True(t, s.NativeDomainSeparatedTransactions.FullSupport)
		assert.Equal(t, 13, s.NativeDomainSeparatedTransactions.SupportCount)
	})

	t.Run("v1_only_null_activation_time", func(t *testing.T) {
		c, _ := newQueryClient(t, `{
			"native_write_mode": "v1_only",
			"read_only": false,
			"activation_source": "not_activated",
			"dual_activated_at_secs": null,
			"native_domain_separated_transactions": {"support_count": 3, "required_count": 13, "full_support": false}
		}`)

		s, err := c.GetNativeWriteStatus(context.Background())
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, NativeWriteModeV1Only, s.NativeWriteMode)
		assert.Equal(t, ActivationSourceNotActivated, s.ActivationSource)
		assert.Nil(t, s.DualActivatedAtSecs)
	})
}

func TestGetPricingPlanByID_Query(t *testing.T) {
	// A plan id with a slash and a space must be path-escaped, not split into
	// extra path segments.
	c, gotURL := newQueryClient(t, `{
		"address": "0x5458747a0efb9ebeb8696fcac1479278c0872fbe",
		"version": "v1",
		"token": null,
		"criteria": [{"type": "sender_token", "address": "0x1111111111111111111111111111111111111111", "token": "0x2222222222222222222222222222222222222222"}],
		"tiers": [{"min_amount": "0", "max_amount": null, "fee": {"type": "defaultratio", "points": 30}}],
		"active_from": 100,
		"active_to": null
	}`)

	plan, err := c.GetPricingPlanByID(context.Background(), "abc/def ghi")
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, *gotURL, "/v1/pricing/plans/abc%2Fdef%20ghi")
	assert.Equal(t, PricingPlanVersionV1, plan.Version)
	assert.Nil(t, plan.Token)
	if assert.Len(t, plan.Criteria, 1) {
		assert.Equal(t, "sender_token", plan.Criteria[0].Type)
		if assert.NotNil(t, plan.Criteria[0].Token) {
			assert.Equal(t, addr("0x2222222222222222222222222222222222222222"), *plan.Criteria[0].Token)
		}
	}
	if assert.Len(t, plan.Tiers, 1) {
		assert.Nil(t, plan.Tiers[0].MaxAmount)
		assert.Equal(t, "defaultratio", plan.Tiers[0].Fee.Type)
		if assert.NotNil(t, plan.Tiers[0].Fee.Points) {
			assert.Equal(t, uint16(30), *plan.Tiers[0].Fee.Points)
		}
	}
	if assert.NotNil(t, plan.ActiveFrom) {
		assert.Equal(t, uint64(100), *plan.ActiveFrom)
	}
	assert.Nil(t, plan.ActiveTo)
}

func TestGetPricingPlans_Query(t *testing.T) {
	t.Run("sender_scope_only", func(t *testing.T) {
		// Only the provided scope becomes a query param; the lookup fields for
		// absent scopes decode to nil.
		c, gotURL := newQueryClient(t, `{
			"sender": {"address": "0x5458747a0efb9ebeb8696fcac1479278c0872fbe", "version": "v0", "token": null, "criteria": [{"type": "sender", "address": "0x1111111111111111111111111111111111111111"}], "tiers": [{"min_amount": "0", "max_amount": "100", "fee": {"type": "fixed", "amount": 5}}], "active_from": null, "active_to": null},
			"recipient": null,
			"token": null
		}`)

		lookup, err := c.GetPricingPlans(context.Background(), "0xsender", "", "")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "/v1/pricing/plans?")
		assert.Contains(t, *gotURL, "sender=0xsender")
		assert.NotContains(t, *gotURL, "recipient=")
		assert.NotContains(t, *gotURL, "token=")
		if assert.NotNil(t, lookup.Sender) {
			assert.Equal(t, PricingPlanVersionV0, lookup.Sender.Version)
			if assert.Len(t, lookup.Sender.Tiers, 1) {
				assert.Equal(t, "fixed", lookup.Sender.Tiers[0].Fee.Type)
				if assert.NotNil(t, lookup.Sender.Tiers[0].Fee.Amount) {
					assert.Equal(t, uint64(5), *lookup.Sender.Tiers[0].Fee.Amount)
				}
				if assert.NotNil(t, lookup.Sender.Tiers[0].MaxAmount) {
					assert.Equal(t, "100", *lookup.Sender.Tiers[0].MaxAmount)
				}
			}
		}
		assert.Nil(t, lookup.Recipient)
		assert.Nil(t, lookup.Token)
	})

	t.Run("all_scopes_sent", func(t *testing.T) {
		c, gotURL := newQueryClient(t, `{"sender": null, "recipient": null, "token": null}`)

		_, err := c.GetPricingPlans(context.Background(), "0xs", "0xr", "0xt")
		if !assert.NoError(t, err) {
			return
		}
		assert.Contains(t, *gotURL, "sender=0xs")
		assert.Contains(t, *gotURL, "recipient=0xr")
		assert.Contains(t, *gotURL, "token=0xt")
	})
}

// -----------------------------------------------------------------------------
// Error paths shared by every query method (handleAPIResponse)
// -----------------------------------------------------------------------------

func TestQueryNon200ReturnsAPIError(t *testing.T) {
	c, _ := newQueryClientStatus(t, http.StatusNotFound, `{"error_code":"NOT_FOUND","message":"no such account"}`)

	_, err := c.GetAccountNonce(context.Background(), addr("0x1111111111111111111111111111111111111111"))
	if !assert.Error(t, err) {
		return
	}
	var apiErr *APIError
	if assert.True(t, errors.As(err, &apiErr), "want *APIError, got %T", err) {
		assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	}
}

func TestQueryMalformedBodyReturnsError(t *testing.T) {
	c, _ := newQueryClient(t, `not json`)

	_, err := c.GetChainId(context.Background())
	assert.Error(t, err, "a 200 with a non-JSON body must surface a decode error")
}
