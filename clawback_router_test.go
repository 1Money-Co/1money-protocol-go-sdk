package onemoney

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

const (
	clawbackRoute     = "/v2/tokens/clawback"
	laneClawbackRoute = "/v2/tokens/lane_clawback"
)

var (
	frozenHolder   = repeatAddr(0x0a)
	clawbackTarget = repeatAddr(0x0b)
)

func clawbackIntent(nonce uint64) TokenClawbackPayload {
	return TokenClawbackPayload{ChainID: 1212101, Nonce: nonce, Token: repeatAddr(0x01), From: frozenHolder, Recipient: clawbackTarget, Value: big.NewInt(500)}
}

// clawbackNode is a node of the release in the settled mode, with the
// signer's next nonce at 3 and the holder's at 4.
func clawbackNode(t *testing.T) *mockNode {
	t.Helper()
	node := newMockNode(t)
	node.setNonces(testSigner(t).Address(), 3)
	node.setNonces(frozenHolder, 4)
	node.setCheckpoints(1200)
	return node
}

type storeLog struct {
	node   *mockNode
	stored []SignedTransaction
}

// hook stores each transaction as a caller would, so every router test also
// checks that the transaction it hands over can be stored and loaded.
func (s *storeLog) hook() OnSigned {
	return func(_ context.Context, signed SignedTransaction) error {
		raw, err := json.Marshal(signed.Transaction)
		if err != nil {
			s.node.t.Errorf("store %s: %v", signed.Transaction.path, err)
			return err
		}
		if _, err := ParseAuthorizedTransaction(raw); err != nil {
			s.node.t.Errorf("load %s: %v", signed.Transaction.path, err)
			return err
		}
		s.stored = append(s.stored, signed)
		s.node.note("store " + signed.Transaction.path)
		return nil
	}
}

func TestClawbackByModeIssuesTheLaneClawbackAtTheHoldersNextNonce(t *testing.T) {
	node := clawbackNode(t)
	store := &storeLog{node: node}
	got, err := node.client().Tokens().ClawbackByMode(context.Background(), clawbackIntent(3), ClawbackFromAccount, testSigner(t), store.hook())
	if err != nil {
		t.Fatal(err)
	}
	posted := node.postedRequests()
	if got.Envelope != AuthorityEnvelopeSettled || got.Lane == nil || got.TakesSignerNonce() || len(posted) != 1 || posted[0].path != laneClawbackRoute {
		t.Fatalf("submission %+v, posted %+v", got, posted)
	}
	if posted[0].body["from_nonce"] != float64(4) || posted[0].body["reference_checkpoint"] != float64(1200) {
		t.Fatalf("body = %v", posted[0].body)
	}
	if got.TransactionHash.Hex() != posted[0].hash || got.Lane.TransactionHash() != got.TransactionHash {
		t.Fatalf("hash %s, posted %s", got.TransactionHash.Hex(), posted[0].hash)
	}
	if len(store.stored) != 1 || store.stored[0].Transaction.hash().Hex() != posted[0].hash {
		t.Fatalf("stored %+v", store.stored)
	}
	events := node.eventLog()
	if store, send := indexOf(events, "store "+laneClawbackRoute), indexOf(events, "POST "+laneClawbackRoute); store < 0 || store > send {
		t.Fatalf("the store must come before the send: %v", events)
	}
}

func indexOf(events []string, event string) int {
	for i, e := range events {
		if e == event {
			return i
		}
	}
	return -1
}

func TestAClawbackThatStaysLegacyReadsNoMode(t *testing.T) {
	verify := clawbackIntent(3)
	verify.Token = VerifyTokenAddress
	cases := []struct {
		name   string
		data   TokenClawbackPayload
		source ClawbackSource
	}{
		{"VERIFY by its master", verify, ClawbackFromAccount},
		{"a Reserved Payment hold", clawbackIntent(3), ClawbackFromHold},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := clawbackNode(t)
			node.setStatus(okJSON(`{"envelope_mode":"upgrade_gap"}`))
			store := &storeLog{node: node}
			got, err := node.client().Tokens().ClawbackByMode(context.Background(), tc.data, tc.source, testSigner(t), store.hook())
			if err != nil {
				t.Fatal(err)
			}
			if got.Envelope != AuthorityEnvelopeLegacy || !got.TakesSignerNonce() || got.Lane != nil {
				t.Fatalf("submission %+v", got)
			}
			want := []string{"store " + clawbackRoute, "POST " + clawbackRoute}
			if events := node.eventLog(); !reflect.DeepEqual(events, want) {
				t.Fatalf("events = %v, want %v", events, want)
			}
		})
	}
}

func TestClawbackByModeInTheGapSignsNothing(t *testing.T) {
	node := clawbackNode(t)
	node.setStatus(okJSON(`{"envelope_mode":"upgrade_gap"}`))
	store := &storeLog{node: node}
	_, err := node.client().Tokens().ClawbackByMode(context.Background(), clawbackIntent(3), ClawbackFromAccount, testSigner(t), store.hook())
	if !errors.Is(err, ErrUpgradeGap) || len(store.stored) != 0 || len(node.postedRequests()) != 0 {
		t.Fatalf("err %v, stored %d, posted %d", err, len(store.stored), len(node.postedRequests()))
	}
}

func TestTheLaneClawbackOfAnIntentNeedsTheSignersNonceNotPassed(t *testing.T) {
	node := clawbackNode(t)
	store := &storeLog{node: node}
	_, err := node.client().Tokens().ClawbackByMode(context.Background(), clawbackIntent(2), ClawbackFromAccount, testSigner(t), store.hook())
	var used *ClawbackIntentNonceUsedError
	if !errors.As(err, &used) || used.Nonce != 2 || used.NextNonce != 3 || used.Signer != testSigner(t).Address() {
		t.Fatalf("err = %v", err)
	}
	if len(store.stored) != 0 || len(node.postedRequests()) != 0 {
		t.Fatalf("stored %d, posted %d", len(store.stored), len(node.postedRequests()))
	}
}

func TestARetiredLegacyClawbackBecomesTheLaneClawback(t *testing.T) {
	node := clawbackNode(t)
	node.queuePost(clawbackRoute, retired())
	store := &storeLog{node: node}
	got, err := node.client().Tokens().SendClawback(context.Background(), AuthorityEnvelopeLegacy, clawbackIntent(3), ClawbackFromAccount, testSigner(t), store.hook())
	if err != nil {
		t.Fatal(err)
	}
	posted := node.postedRequests()
	if got.Lane == nil || len(posted) != 2 || posted[1].path != laneClawbackRoute {
		t.Fatalf("submission %+v, posted %+v", got, posted)
	}
	if len(store.stored) != 2 || store.stored[0].Replaces != nil || store.stored[1].Replaces.Hex() != posted[0].hash {
		t.Fatalf("stored %+v", store.stored)
	}
	want := []string{
		"store " + clawbackRoute, "POST " + clawbackRoute, "GET /api/status", "GET " + endpointAccountsNonce,
		"GET " + endpointAccountsNonce, "GET " + endpointCheckpointsNumber, "store " + laneClawbackRoute, "POST " + laneClawbackRoute,
	}
	if events := node.eventLog(); !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v\nwant %v", events, want)
	}
}

func TestARetiredLegacyClawbackIsReturnedWhenItStaysLegacyOrTheNodeIsNotSettled(t *testing.T) {
	node := clawbackNode(t)
	node.queuePost(clawbackRoute, retired())
	_, err := node.client().Tokens().SendClawback(context.Background(), AuthorityEnvelopeLegacy, clawbackIntent(3), ClawbackFromHold, testSigner(t), (&storeLog{node: node}).hook())
	if !isRefusal(err, RefusalLegacyEnvelopeRetired) || len(node.postedRequests()) != 1 || indexOf(node.eventLog(), "GET /api/status") >= 0 {
		t.Fatalf("hold: err %v, events %v", err, node.eventLog())
	}

	node = clawbackNode(t)
	node.setStatus(okJSON(`{}`))
	node.queuePost(clawbackRoute, retired())
	_, err = node.client().Tokens().SendClawback(context.Background(), AuthorityEnvelopeLegacy, clawbackIntent(3), ClawbackFromAccount, testSigner(t), (&storeLog{node: node}).hook())
	if !isRefusal(err, RefusalLegacyEnvelopeRetired) || len(node.postedRequests()) != 1 {
		t.Fatalf("no mode: err %v, posted %d", err, len(node.postedRequests()))
	}
}

func TestTheClawbackRouterNeedsAHookAndTheV2Mode(t *testing.T) {
	node := clawbackNode(t)
	if _, err := node.client().Tokens().ClawbackByMode(context.Background(), clawbackIntent(3), ClawbackFromAccount, testSigner(t), nil); err == nil || !strings.Contains(err.Error(), "onSigned") {
		t.Fatalf("nil hook: %v", err)
	}
	legacyV1 := node.client(WithLegacyV1())
	hook := (&storeLog{node: node}).hook()
	calls := map[string]func() error{
		"ClawbackByMode": func() error {
			_, err := legacyV1.Tokens().ClawbackByMode(context.Background(), clawbackIntent(3), ClawbackFromAccount, testSigner(t), hook)
			return err
		},
		"SendClawback": func() error {
			_, err := legacyV1.Tokens().SendClawback(context.Background(), AuthorityEnvelopeLegacy, clawbackIntent(3), ClawbackFromHold, testSigner(t), hook)
			return err
		},
		"IssueLaneClawback": func() error {
			_, err := legacyV1.Tokens().IssueLaneClawback(context.Background(), clawbackIntent(3), testSigner(t), hook)
			return err
		},
	}
	for name, call := range calls {
		if err := call(); err == nil || !strings.Contains(err.Error(), "legacy v1") {
			t.Errorf("%s in legacy v1 mode: %v", name, err)
		}
	}
	if len(node.eventLog()) != 0 {
		t.Fatalf("a refused call reached the node: %v", node.eventLog())
	}
}

func TestAClawbackThatCannotBeStoredIsNotSent(t *testing.T) {
	for _, envelope := range []AuthorityEnvelope{AuthorityEnvelopeLegacy, AuthorityEnvelopeSettled} {
		node := clawbackNode(t)
		storeFailed := errors.New("disk full")
		_, err := node.client().Tokens().SendClawback(context.Background(), envelope, clawbackIntent(3), ClawbackFromAccount, testSigner(t),
			func(context.Context, SignedTransaction) error { return storeFailed })
		if !errors.Is(err, storeFailed) || len(node.postedRequests()) != 0 {
			t.Fatalf("%s: err %v, posted %d", envelope, err, len(node.postedRequests()))
		}
	}
}

func TestALaneClawbackRefusedWithAReferenceNotReadyIsBuiltOnceMore(t *testing.T) {
	referenceRetryDelayForTest(t)
	node := clawbackNode(t)
	node.setCheckpoints(1200, 1201)
	node.queuePost(laneClawbackRoute, problemReply(http.StatusUnprocessableEntity, "reference_checkpoint_not_committed", "retry_later", map[string]any{"checkpoint_id": 1200, "checkpoint_id_latest": 1199}))
	store := &storeLog{node: node}
	record, err := node.client().Tokens().IssueLaneClawback(context.Background(), clawbackIntent(3), testSigner(t), store.hook())
	if err != nil {
		t.Fatal(err)
	}
	posted := node.postedRequests()
	if len(posted) != 2 || posted[1].body["reference_checkpoint"] != float64(1201) || record.TransactionHash().Hex() != posted[1].hash {
		t.Fatalf("posted %+v", posted)
	}
	if len(store.stored) != 2 || store.stored[1].Replaces.Hex() != posted[0].hash {
		t.Fatalf("stored %+v", store.stored)
	}
}

func TestAfterAFailedSendTheStoredClawbackIsTheOneToRead(t *testing.T) {
	node := clawbackNode(t)
	node.queuePost(laneClawbackRoute, mockReply{status: http.StatusInternalServerError, body: `{}`})
	store := &storeLog{node: node}
	_, err := node.client().Tokens().ClawbackByMode(context.Background(), clawbackIntent(3), ClawbackFromAccount, testSigner(t), store.hook())
	if err == nil || len(store.stored) != 1 {
		t.Fatalf("err %v, stored %d", err, len(store.stored))
	}
	raw, err := json.Marshal(store.stored[0].Transaction)
	if err != nil {
		t.Fatal(err)
	}
	record, err := ParseLaneClawbackRecord(raw)
	if err != nil || record.TransactionHash().Hex() != node.postedRequests()[0].hash {
		t.Fatalf("record %v, err %v", record, err)
	}
	node.setReceipt(record.TransactionHash().Hex(), okJSON(`{"success":true,"transaction_hash":"`+record.TransactionHash().Hex()+`"}`))
	fate, err := node.client().LaneClawbackFate(context.Background(), record)
	if err != nil || fate.Kind != LaneClawbackExecuted {
		t.Fatalf("fate %+v, err %v", fate, err)
	}
}

// lyingSigner reports another key's address.
type lyingSigner struct{ Signer }

func (lyingSigner) Address() common.Address { return repeatAddr(0x77) }

func TestASignerThatReportsAnotherAddressStoresAndSendsNothing(t *testing.T) {
	node := clawbackNode(t)
	node.setNonces(repeatAddr(0x77), 3)
	store := &storeLog{node: node}
	_, err := node.client().Tokens().ClawbackByMode(context.Background(), clawbackIntent(3), ClawbackFromAccount, lyingSigner{testSigner(t)}, store.hook())
	if err == nil || !strings.Contains(err.Error(), "signs with the key of") || len(store.stored) != 0 || len(node.postedRequests()) != 0 {
		t.Fatalf("err %v, stored %d, posted %d", err, len(store.stored), len(node.postedRequests()))
	}
}

func TestPrepareLaneClawbackReissue(t *testing.T) {
	dead := signedLaneRecord(t, laneVectorPayload())
	hash := dead.TransactionHash().Hex()
	holder := dead.Payload().Holder
	refusals := []struct {
		name    string
		nonces  []uint64
		receipt *mockReply
		status  *mockReply
		want    LaneClawbackFateKind
	}{
		{"executed", []uint64{8}, ptr(okJSON(`{"success":true,"transaction_hash":"` + hash + `"}`)), nil, LaneClawbackExecuted},
		{"certified", []uint64{8}, nil, ptr(statusReplyFor(hash, TransactionStatusFinalized)), LaneClawbackCertified},
		{"slot not passed", []uint64{7}, nil, nil, LaneClawbackPending},
		{"a node knows its hash", []uint64{8}, nil, ptr(statusReplyFor(hash, TransactionStatusPending)), LaneClawbackUnknown},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			node := newMockNode(t)
			node.setNonces(holder, tc.nonces...)
			if tc.receipt != nil {
				node.setReceipt(hash, *tc.receipt)
			}
			if tc.status != nil {
				node.setTxStatus(hash, *tc.status)
			}
			reissue, err := node.client().PrepareLaneClawbackReissue(context.Background(), dead, testSigner(t), FullHistoryChecked)
			if err != nil || reissue.Kind != ReissueRefused || reissue.Fate.Kind != tc.want || reissue.Replacement != nil {
				t.Fatalf("reissue %+v, err %v", reissue, err)
			}
		})
	}

	t.Run("node behind", func(t *testing.T) {
		node := newMockNode(t)
		node.setNonces(holder, 8, 7)
		reissue, err := node.client().PrepareLaneClawbackReissue(context.Background(), dead, testSigner(t), FullHistoryChecked)
		if err != nil || reissue.Kind != ReissueNodeBehind || reissue.Replacement != nil {
			t.Fatalf("reissue %+v, err %v", reissue, err)
		}
	})

	t.Run("prepared on the next slot, not sent", func(t *testing.T) {
		node := newMockNode(t)
		node.setNonces(holder, 8)
		node.setCheckpoints(1300)
		reissue, err := node.client().PrepareLaneClawbackReissue(context.Background(), dead, testSigner(t), FullHistoryChecked)
		if err != nil || reissue.Kind != ReissuePrepared {
			t.Fatalf("reissue %+v, err %v", reissue, err)
		}
		replacement := reissue.Replacement.Payload()
		old := dead.Payload()
		if replacement.FromNonce != 8 || replacement.ReferenceCheckpoint != 1300 || replacement.Value.Cmp(old.Value) != 0 ||
			replacement.Recipient != old.Recipient || replacement.Token != old.Token || reissue.Replacement.Memo() != dead.Memo() {
			t.Fatalf("replacement %+v / %+v", replacement, reissue.Replacement.Memo())
		}
		if len(node.postedRequests()) != 0 {
			t.Fatal("the replacement was sent")
		}
	})

	t.Run("needs the full-history statement", func(t *testing.T) {
		node := newMockNode(t)
		if _, err := node.client().PrepareLaneClawbackReissue(context.Background(), dead, testSigner(t), FullHistoryCheck{}); err == nil || len(node.eventLog()) != 0 {
			t.Fatalf("err %v, events %v", err, node.eventLog())
		}
	})
}

// failOnSecondStore stores the first signed transaction and fails on the second.
func failOnSecondStore(storeFailed error) OnSigned {
	calls := 0
	return func(context.Context, SignedTransaction) error {
		calls++
		if calls == 2 {
			return storeFailed
		}
		return nil
	}
}

func TestAStoreThatFailsOnTheSecondSignatureSendsNothingMore(t *testing.T) {
	referenceRetryDelayForTest(t)
	storeFailed := errors.New("disk full")

	t.Run("the lane clawback after a retired legacy clawback", func(t *testing.T) {
		node := clawbackNode(t)
		node.queuePost(clawbackRoute, retired())
		_, err := node.client().Tokens().SendClawback(context.Background(), AuthorityEnvelopeLegacy, clawbackIntent(3), ClawbackFromAccount, testSigner(t), failOnSecondStore(storeFailed))
		posted := node.postedRequests()
		if !errors.Is(err, storeFailed) || len(posted) != 1 || posted[0].path != clawbackRoute {
			t.Fatalf("err %v, posted %+v", err, posted)
		}
	})

	t.Run("the retry lane clawback after a refused reference", func(t *testing.T) {
		node := clawbackNode(t)
		node.setCheckpoints(1200, 1201)
		node.queuePost(laneClawbackRoute, problemReply(http.StatusUnprocessableEntity, "token_not_at_reference_checkpoint", "retry_later", map[string]any{"mint_address": repeatAddr(1).Hex(), "checkpoint_id": 1200}))
		_, err := node.client().Tokens().IssueLaneClawback(context.Background(), clawbackIntent(3), testSigner(t), failOnSecondStore(storeFailed))
		posted := node.postedRequests()
		if !errors.Is(err, storeFailed) || len(posted) != 1 || posted[0].path != laneClawbackRoute {
			t.Fatalf("err %v, posted %+v", err, posted)
		}
	})
}
