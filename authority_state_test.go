package onemoney

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	signerAccount = common.HexToAddress("0x5555555555555555555555555555555555555555")
	stateHash     = common.HexToHash(someHash)
)

func statusReply(status TransactionStatus) mockReply { return statusReplyFor(someHash, status) }

func statusReplyFor(hash string, status TransactionStatus) mockReply {
	return okJSON(fmt.Sprintf(`{"hash":%q,"status":%q}`, hash, status))
}

const otherHash = "0x2222222222222222222222222222222222222222222222222222222222222222"

// answersForAnotherTransaction are 200 answers that name no transaction or
// another one: no evidence about the transaction that was asked for.
func answersForAnotherTransaction() map[string]struct{ receipt, status mockReply } {
	return map[string]struct{ receipt, status mockReply }{
		"receipt {}":                   {okJSON(`{}`), notFound},
		"receipt null":                 {okJSON(`null`), notFound},
		"receipt of another hash":      {okJSON(`{"success":false,"transaction_hash":"` + otherHash + `"}`), notFound},
		"status {}":                    {notFound, okJSON(`{}`)},
		"status null":                  {notFound, okJSON(`null`)},
		"status of another hash":       {notFound, statusReplyFor(otherHash, TransactionStatusCheckpointed)},
		"pending status, another hash": {notFound, statusReplyFor(otherHash, TransactionStatusPending)},
	}
}

func TestAuthorityOperationStateConcludesOnlyFromPositiveEvidence(t *testing.T) {
	receipt := okJSON(`{"success":true,"transaction_hash":"` + someHash + `"}`)
	cases := []struct {
		name      string
		nextNonce uint64
		receipt   *mockReply
		status    *mockReply
		want      AuthorityOperationStateKind
		unknown   *UnknownOutcome
	}{
		{"receipt decides", 9, &receipt, nil, AuthorityDecided, nil},
		{"counter-signed is settling", 9, nil, ptr(statusReply(TransactionStatusCertifiedPendingCheckpoint)), AuthoritySettling, nil},
		{"checkpointed is settling", 9, nil, ptr(statusReply(TransactionStatusCheckpointed)), AuthoritySettling, nil},
		{"a certificate wins over a nonce that is behind", 3, nil, ptr(statusReply(TransactionStatusFinalized)), AuthoritySettling, nil},
		{"nonce not passed", 5, nil, nil, AuthorityPending, nil},
		{"nonce not passed, pending status", 5, nil, ptr(statusReply(TransactionStatusPending)), AuthorityPending, nil},
		{"nonce passed, no receipt, no status", 6, nil, nil, AuthorityUnknown, &UnknownOutcome{NextNonce: 6}},
		{"nonce passed, pending status", 6, nil, ptr(statusReply(TransactionStatusPending)), AuthorityUnknown, &UnknownOutcome{NextNonce: 6, Status: ptr(TransactionStatusPending)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newMockNode(t)
			node.setNonces(signerAccount, tc.nextNonce)
			if tc.receipt != nil {
				node.setReceipt(someHash, *tc.receipt)
			}
			if tc.status != nil {
				node.setTxStatus(someHash, *tc.status)
			}
			client := node.client()
			for round := 0; round < 3; round++ {
				state, err := client.AuthorityOperationState(context.Background(), signerAccount, 5, stateHash)
				if err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
				if state.Kind != tc.want {
					t.Fatalf("round %d: kind = %s, want %s", round, state.Kind, tc.want)
				}
				if tc.want == AuthorityDecided && (state.Receipt == nil || !state.Receipt.Success) {
					t.Fatalf("decided without its receipt: %+v", state)
				}
				if tc.unknown != nil && (state.Unknown == nil || state.Unknown.NextNonce != tc.unknown.NextNonce ||
					!equalPtr(state.Unknown.Status, tc.unknown.Status)) {
					t.Fatalf("unknown = %+v, want %+v", state.Unknown, tc.unknown)
				}
			}
		})
	}
}

func TestAuthorityOperationStateReadsTheNonceFirst(t *testing.T) {
	node := newMockNode(t)
	node.setNonces(signerAccount, 6)
	if _, err := node.client().AuthorityOperationState(context.Background(), signerAccount, 5, stateHash); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET " + endpointAccountsNonce, "GET " + endpointTransactionsReceiptByHashV1, "GET " + endpointTransactionsStatusByHashV1}
	if got := node.eventLog(); !slices.Equal(got, want) {
		t.Fatalf("reads = %v, want %v", got, want)
	}
}

func TestAuthorityOperationStateErrorsOnAReadItCannotUse(t *testing.T) {
	cases := []struct {
		name            string
		receipt, status mockReply
	}{
		{"receipt read fails", mockReply{status: http.StatusInternalServerError, body: `{}`}, notFound},
		{"receipt does not decode", okJSON(`{"success":"maybe"}`), notFound},
		{"status read fails", notFound, mockReply{status: http.StatusServiceUnavailable, body: `{}`}},
		{"unknown status", notFound, statusReply("evicted")},
		{"receipt read throttled with a 404", problemReply(http.StatusNotFound, "future_kind", "retry_later", nil), notFound},
		{"status read throttled with a 404", notFound, problemReply(http.StatusNotFound, "future_kind", "retry_later", nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newMockNode(t)
			node.setNonces(signerAccount, 9)
			node.setReceipt(someHash, tc.receipt)
			node.setTxStatus(someHash, tc.status)
			state, err := node.client().AuthorityOperationState(context.Background(), signerAccount, 5, stateHash)
			if err == nil {
				t.Fatalf("state = %+v, want an error", state)
			}
		})
	}
}

func signedLaneRecord(t *testing.T, payload LaneClawbackPayload) *LaneClawbackRecord {
	t.Helper()
	record, err := NewLaneClawbackRecord(mustAuthorize(t, payload, WithMemo(Memo{Type: "case", Format: "text", Data: "7"})))
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestLaneClawbackRecord(t *testing.T) {
	record := signedLaneRecord(t, laneVectorPayload())
	if record.Payload().FromNonce != 7 || record.Memo().Data != "7" || record.Payload().Value.Cmp(oneToken()) != 0 {
		t.Fatalf("record = %+v / %+v", record.Payload(), record.Memo())
	}
	stored, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ParseLaneClawbackRecord(stored)
	if err != nil || loaded.TransactionHash() != record.TransactionHash() {
		t.Fatalf("loaded %v, err %v", loaded, err)
	}
	clawback := mustAuthorize(t, TokenClawbackPayload{ChainID: 1, Nonce: 1, Token: repeatAddr(1), From: repeatAddr(2), Recipient: repeatAddr(3), Value: big.NewInt(1)})
	if _, err := NewLaneClawbackRecord(clawback); err == nil {
		t.Fatal("a legacy clawback is not a lane clawback record")
	}
	stored, _ = json.Marshal(clawback)
	if _, err := ParseLaneClawbackRecord(stored); err == nil || !strings.Contains(err.Error(), "not a lane clawback") {
		t.Fatalf("parse of a legacy clawback: %v", err)
	}
}

func TestLaneClawbackFate(t *testing.T) {
	record := signedLaneRecord(t, laneVectorPayload())
	hash := record.TransactionHash().Hex()
	holder := record.Payload().Holder
	statusReply := func(status TransactionStatus) mockReply { return statusReplyFor(hash, status) }
	cases := []struct {
		name      string
		nextNonce uint64
		receipt   *mockReply
		status    *mockReply
		want      LaneClawbackFateKind
	}{
		{"executed", 8, ptr(okJSON(`{"success":true,"transaction_hash":"0x` + strings.ToUpper(hash[2:]) + `"}`)), nil, LaneClawbackExecuted},
		{"certified", 8, nil, ptr(statusReply(TransactionStatusFinalized)), LaneClawbackCertified},
		{"pending", 7, nil, nil, LaneClawbackPending},
		{"unknown", 8, nil, nil, LaneClawbackUnknown},
		{"unknown with a pending status", 8, nil, ptr(statusReply(TransactionStatusPending)), LaneClawbackUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newMockNode(t)
			node.setNonces(holder, tc.nextNonce)
			if tc.receipt != nil {
				node.setReceipt(hash, *tc.receipt)
			}
			if tc.status != nil {
				node.setTxStatus(hash, *tc.status)
			}
			fate, err := node.client().LaneClawbackFate(context.Background(), record)
			if err != nil || fate.Kind != tc.want {
				t.Fatalf("fate = %+v, err = %v, want %s", fate, err, tc.want)
			}
		})
	}
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestLaneClawbackPayloadUsesTheHoldersNextNonce(t *testing.T) {
	node := newMockNode(t)
	holder := repeatAddr(0x02)
	node.setNonces(holder, 4)
	node.setCheckpoints(1200)
	payload, err := node.client().LaneClawbackPayload(context.Background(), 1212101, repeatAddr(1), holder, repeatAddr(3), big.NewInt(500))
	if err != nil {
		t.Fatal(err)
	}
	want := LaneClawbackPayload{ChainID: 1212101, Token: repeatAddr(1), Holder: holder, FromNonce: 4, ReferenceCheckpoint: 1200, Recipient: repeatAddr(3), Value: big.NewInt(500)}
	if payload.FromNonce != want.FromNonce || payload.ReferenceCheckpoint != want.ReferenceCheckpoint || payload.Holder != holder || payload.Value.Cmp(want.Value) != 0 {
		t.Fatalf("payload = %+v", payload)
	}

	if _, err := node.client().LaneClawbackPayload(context.Background(), 1212101, VerifyTokenAddress, holder, repeatAddr(3), big.NewInt(1)); err == nil || !strings.Contains(err.Error(), "VERIFY") {
		t.Fatalf("VERIFY: %v", err)
	}
	if got := len(node.eventLog()); got != 2 {
		t.Fatalf("the VERIFY refusal read the node: %d reads", got-2)
	}

	newcomer := repeatAddr(0x09)
	node.setNonces(newcomer, 0)
	if _, err := node.client().LaneClawbackPayload(context.Background(), 1212101, repeatAddr(1), newcomer, repeatAddr(3), big.NewInt(1)); err == nil || !strings.Contains(err.Error(), "no lane yet") {
		t.Fatalf("holder without a lane: %v", err)
	}
}

// A node answer names the transaction with the same hash rule as a submit:
// any case, with "0x", "0X" or no prefix.
func TestAReadNamesTheTransactionInAnyHashForm(t *testing.T) {
	digits := someHash[2:]
	for _, form := range []string{digits, "0X" + digits, "0x" + strings.ToUpper(digits)} {
		node := newMockNode(t)
		node.setNonces(signerAccount, 9)
		node.setReceipt(someHash, okJSON(`{"success":true,"transaction_hash":"`+form+`"}`))
		state, err := node.client().AuthorityOperationState(context.Background(), signerAccount, 5, stateHash)
		if err != nil || state.Kind != AuthorityDecided {
			t.Fatalf("receipt %s: state = %+v, err = %v", form, state, err)
		}

		node = newMockNode(t)
		node.setNonces(signerAccount, 9)
		node.setReceipt(someHash, notFound)
		node.setTxStatus(someHash, statusReplyFor(form, TransactionStatusCheckpointed))
		state, err = node.client().AuthorityOperationState(context.Background(), signerAccount, 5, stateHash)
		if err != nil || state.Kind != AuthoritySettling {
			t.Fatalf("status %s: state = %+v, err = %v", form, state, err)
		}
	}
	for _, bad := range []string{someHash[:65], "0x" + strings.Repeat("g", 64)} {
		node := newMockNode(t)
		node.setNonces(signerAccount, 9)
		node.setReceipt(someHash, okJSON(`{"success":true,"transaction_hash":"`+bad+`"}`))
		if _, err := node.client().AuthorityOperationState(context.Background(), signerAccount, 5, stateHash); err == nil || !strings.Contains(err.Error(), "not for this transaction") {
			t.Fatalf("receipt %s: err = %v", bad, err)
		}
	}
}

func TestAReadForAnotherTransactionIsNoEvidence(t *testing.T) {
	record := signedLaneRecord(t, laneVectorPayload())
	for name, answer := range answersForAnotherTransaction() {
		t.Run("authority operation, "+name, func(t *testing.T) {
			node := newMockNode(t)
			node.setNonces(signerAccount, 5)
			node.setReceipt(someHash, answer.receipt)
			node.setTxStatus(someHash, answer.status)
			state, err := node.client().AuthorityOperationState(context.Background(), signerAccount, 5, stateHash)
			if err == nil || !strings.Contains(err.Error(), "not for this transaction") {
				t.Fatalf("state = %+v, err = %v; want a read error", state, err)
			}
		})
		t.Run("lane clawback, "+name, func(t *testing.T) {
			node := newMockNode(t)
			node.setNonces(record.Payload().Holder, 7)
			node.setReceipt(record.TransactionHash().Hex(), answer.receipt)
			node.setTxStatus(record.TransactionHash().Hex(), answer.status)
			fate, err := node.client().LaneClawbackFate(context.Background(), record)
			if err == nil || !strings.Contains(err.Error(), "not for this transaction") {
				t.Fatalf("fate = %+v, err = %v; want a read error", fate, err)
			}
		})
	}
}
