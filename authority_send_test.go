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

func TestSettledOperationOfALegacyPayload(t *testing.T) {
	token := repeatAddr(0x01)
	account := repeatAddr(0x02)
	authority := func(action AuthorityAction, kind AuthorityType, address common.Address) LegacyAuthorityOperation {
		return LegacyAuthority(TokenAuthorityPayload{ChainID: 1, Nonce: 4, Action: action, AuthorityType: kind, AuthorityAddress: address, Token: token, Value: big.NewInt(77)})
	}
	list := func(kind ManageListKind, action ManageListActionType) LegacyAuthorityOperation {
		payload := TokenManageListPayload{ChainID: 1, Nonce: 4, Action: action, Address: account, Token: token}
		if kind == ManageListBlacklist {
			return LegacyBlacklist(payload)
		}
		return LegacyWhitelist(payload)
	}
	cases := []struct {
		name    string
		op      LegacyAuthorityOperation
		private bool
		want    SettledAuthorityOperation
	}{
		{"mint", LegacyMint(TokenMintPayload{ChainID: 1, Nonce: 4, Recipient: account, Value: big.NewInt(5), Token: token}), false, SettledMint{Recipient: account, Value: big.NewInt(5)}},
		{"grant master", authority(AuthorityActionGrant, AuthorityTypeMasterMintBurn, account), false, SettledRotateMasterMintBurn{Account: account}},
		{"grant master to zero", authority(AuthorityActionGrant, AuthorityTypeMasterMintBurn, common.Address{}), false, SettledRevokeMasterMintBurn{}},
		{"revoke master", authority(AuthorityActionRevoke, AuthorityTypeMasterMintBurn, account), false, SettledRevokeMasterMintBurn{}},
		{"grant minter", authority(AuthorityActionGrant, AuthorityTypeMintBurnTokens, account), false, SettledSetMinterAllowance{Minter: account, Allowance: big.NewInt(77)}},
		{"revoke minter", authority(AuthorityActionRevoke, AuthorityTypeMintBurnTokens, account), false, SettledRevokeMinter{Minter: account}},
		{"grant pause", authority(AuthorityActionGrant, AuthorityTypePause, account), false, SettledGrantRole{Role: SettledRolePause, Account: account}},
		{"revoke manage list", authority(AuthorityActionRevoke, AuthorityTypeManageList, account), false, SettledRevokeRole{Role: SettledRoleManageList, Account: account}},
		{"grant update metadata", authority(AuthorityActionGrant, AuthorityTypeUpdateMetadata, account), false, SettledGrantRole{Role: SettledRoleUpdateMetadata, Account: account}},
		{"grant bridge", authority(AuthorityActionGrant, AuthorityTypeBridge, account), false, SettledGrantRole{Role: SettledRoleBridge, Account: account}},
		{"revoke clawback", authority(AuthorityActionRevoke, AuthorityTypeClawback, account), false, SettledRevokeRole{Role: SettledRoleClawback, Account: account}},
		{"freeze public", list(ManageListBlacklist, ManageListActionAdd), false, SettledList{List: SettledListBlacklist, Action: SettledListAdd, Account: account}},
		{"unfreeze public", list(ManageListBlacklist, ManageListActionRemove), false, SettledList{List: SettledListBlacklist, Action: SettledListRemove, Account: account}},
		{"blacklist add private", list(ManageListBlacklist, ManageListActionAdd), true, SettledList{List: SettledListWhitelist, Action: SettledListRemove, Account: account}},
		{"blacklist remove private", list(ManageListBlacklist, ManageListActionRemove), true, SettledList{List: SettledListWhitelist, Action: SettledListAdd, Account: account}},
		{"whitelist add public", list(ManageListWhitelist, ManageListActionAdd), false, SettledList{List: SettledListBlacklist, Action: SettledListRemove, Account: account}},
		{"whitelist remove public", list(ManageListWhitelist, ManageListActionRemove), false, SettledList{List: SettledListBlacklist, Action: SettledListAdd, Account: account}},
		{"whitelist add private", list(ManageListWhitelist, ManageListActionAdd), true, SettledList{List: SettledListWhitelist, Action: SettledListAdd, Account: account}},
		{"whitelist remove private", list(ManageListWhitelist, ManageListActionRemove), true, SettledList{List: SettledListWhitelist, Action: SettledListRemove, Account: account}},
		{"pause", LegacyPause(PauseTokenPayload{ChainID: 1, Nonce: 4, Action: Pause, Token: token}), false, SettledPause{}},
		{"unpause", LegacyPause(PauseTokenPayload{ChainID: 1, Nonce: 4, Action: UnPause, Token: token}), false, SettledUnpause{}},
		{"metadata", LegacyMetadata(UpdateMetadataPayload{ChainID: 1, Nonce: 4, Name: "N", URI: "u", Token: token, AdditionalMetadata: []AdditionalMetadata{{Key: "k", Value: "v"}}}), false, SettledUpdateMetadata{Name: "N", URI: "u", AdditionalMetadata: []AdditionalMetadata{{Key: "k", Value: "v"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := tc.op.Settled(tc.private, 42)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(payload.Operation, tc.want) {
				t.Fatalf("operation = %#v, want %#v", payload.Operation, tc.want)
			}
			if payload.ChainID != 1 || payload.Nonce != 4 || payload.Token != token || payload.ReferenceCheckpoint != 42 {
				t.Fatalf("header = %+v", payload)
			}
		})
	}
	for _, bad := range []LegacyAuthorityOperation{
		authority("Steal", AuthorityTypePause, account),
		authority(AuthorityActionGrant, "Owner", account),
		list(ManageListBlacklist, "Toggle"),
		LegacyPause(PauseTokenPayload{Action: "Freeze"}),
		{},
	} {
		if _, err := bad.Settled(false, 42); err == nil {
			t.Errorf("%+v: want an error", bad)
		}
	}
}

func legacyPause(nonce uint64) LegacyAuthorityOperation {
	return LegacyPause(PauseTokenPayload{ChainID: 1212101, Nonce: nonce, Action: Pause, Token: repeatAddr(0x01)})
}

// storeHook records every handed-over transaction in the node's event log, so
// a test can check that each store happens before its send.
func storeHook(node *mockNode, stored *[]SignedTransaction) SubmitOption {
	return WithOnSigned(func(_ context.Context, signed SignedTransaction) error {
		*stored = append(*stored, signed)
		node.note("store " + signed.Transaction.path)
		return nil
	})
}

const (
	authorityRoute = "/v2/tokens/authority_operation"
	pauseRoute     = "/v2/tokens/pause"
)

func retired() mockReply {
	return problemReply(http.StatusUnprocessableEntity, "legacy_envelope_retired", "retry_later", nil)
}

func TestAuthorityOperationByModeFollowsTheSwitchPoint(t *testing.T) {
	t.Run("settled sends the new envelope at the same nonce", func(t *testing.T) {
		node := newMockNode(t)
		node.setCheckpoints(1200)
		got, err := node.client().Tokens().AuthorityOperationByMode(context.Background(), legacyPause(11), testSigner(t))
		if err != nil {
			t.Fatal(err)
		}
		posted := node.postedRequests()
		if got.Envelope != AuthorityEnvelopeSettled || len(posted) != 1 || posted[0].path != authorityRoute {
			t.Fatalf("submission %+v, posted %+v", got, posted)
		}
		if posted[0].body["nonce"] != float64(11) || posted[0].body["reference_checkpoint"] != float64(1200) {
			t.Fatalf("body = %v", posted[0].body)
		}
		if got.TransactionHash.Hex() != posted[0].hash {
			t.Fatalf("hash %s, posted %s", got.TransactionHash.Hex(), posted[0].hash)
		}
	})
	t.Run("no reported mode sends the legacy envelope", func(t *testing.T) {
		node := newMockNode(t)
		node.setStatus(okJSON(`{"native_write_mode":"v2_only"}`))
		got, err := node.client().Tokens().AuthorityOperationByMode(context.Background(), legacyPause(11), testSigner(t))
		posted := node.postedRequests()
		if err != nil || got.Envelope != AuthorityEnvelopeLegacy || len(posted) != 1 || posted[0].path != pauseRoute {
			t.Fatalf("submission %+v, err %v, posted %+v", got, err, posted)
		}
	})
	t.Run("the gap sends nothing", func(t *testing.T) {
		node := newMockNode(t)
		node.setStatus(okJSON(`{"envelope_mode":"upgrade_gap"}`))
		_, err := node.client().Tokens().AuthorityOperationByMode(context.Background(), legacyPause(11), testSigner(t))
		if !errors.Is(err, ErrUpgradeGap) || len(node.postedRequests()) != 0 {
			t.Fatalf("err %v, posted %d", err, len(node.postedRequests()))
		}
	})
	t.Run("an unknown mode sends nothing", func(t *testing.T) {
		node := newMockNode(t)
		node.setStatus(okJSON(`{"envelope_mode":"turbo"}`))
		_, err := node.client().Tokens().AuthorityOperationByMode(context.Background(), legacyPause(11), testSigner(t))
		var unrecognized *UnrecognizedEnvelopeModeError
		if !errors.As(err, &unrecognized) || len(node.postedRequests()) != 0 {
			t.Fatalf("err %v, posted %d", err, len(node.postedRequests()))
		}
	})
}

func TestARetiredLegacyEnvelopeIsSignedAgainAsSettledAtTheSameNonce(t *testing.T) {
	node := newMockNode(t)
	node.setStatus(okJSON(`{"envelope_mode":"settled"}`))
	node.queuePost(pauseRoute, retired())
	var stored []SignedTransaction
	got, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeLegacy, legacyPause(11), testSigner(t), storeHook(node, &stored))
	if err != nil {
		t.Fatal(err)
	}
	posted := node.postedRequests()
	if got.Envelope != AuthorityEnvelopeSettled || len(posted) != 2 || posted[1].path != authorityRoute || posted[1].body["nonce"] != float64(11) {
		t.Fatalf("submission %+v, posted %+v", got, posted)
	}
	if len(stored) != 2 || stored[0].Replaces != nil || stored[1].Replaces == nil || stored[1].Replaces.Hex() != posted[0].hash {
		t.Fatalf("stored = %+v", stored)
	}
	wantOrder := []string{"store " + pauseRoute, "POST " + pauseRoute, "GET /api/status", "GET " + endpointCheckpointsNumber, "store " + authorityRoute, "POST " + authorityRoute}
	if got := node.eventLog(); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("events = %v\nwant %v", got, wantOrder)
	}
}

func TestARetiredLegacyEnvelopeIsReturnedUnlessTheNodeReportsSettled(t *testing.T) {
	for _, status := range []mockReply{okJSON(`{}`), {status: http.StatusNotFound, body: `{}`}} {
		node := newMockNode(t)
		node.setStatus(status)
		node.queuePost(pauseRoute, retired())
		_, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeLegacy, legacyPause(11), testSigner(t))
		if !isRefusal(err, RefusalLegacyEnvelopeRetired) || len(node.postedRequests()) != 1 {
			t.Fatalf("err %v, posted %d", err, len(node.postedRequests()))
		}
	}
	node := newMockNode(t)
	node.setStatus(okJSON(`{"envelope_mode":"upgrade_gap"}`))
	node.queuePost(pauseRoute, retired())
	_, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeLegacy, legacyPause(11), testSigner(t))
	if !errors.Is(err, ErrUpgradeGap) || len(node.postedRequests()) != 1 {
		t.Fatalf("gap: err %v, posted %d", err, len(node.postedRequests()))
	}
}

func TestTheSettledEnvelopeIsRetriedOnceWithAFreshReference(t *testing.T) {
	referenceRetryDelayForTest(t)
	notCommitted := problemReply(http.StatusUnprocessableEntity, "reference_checkpoint_not_committed", "retry_later", map[string]any{"checkpoint_id": 1200, "checkpoint_id_latest": 1199})
	notAtReference := problemReply(http.StatusUnprocessableEntity, "token_not_at_reference_checkpoint", "retry_later", map[string]any{"mint_address": repeatAddr(1).Hex(), "checkpoint_id": 1200})
	for _, refusal := range []mockReply{notCommitted, notAtReference} {
		node := newMockNode(t)
		node.setCheckpoints(1200, 1201)
		node.queuePost(authorityRoute, refusal)
		var stored []SignedTransaction
		got, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeSettled, legacyPause(11), testSigner(t), storeHook(node, &stored))
		posted := node.postedRequests()
		if err != nil || len(posted) != 2 || posted[1].body["reference_checkpoint"] != float64(1201) || posted[1].body["nonce"] != float64(11) {
			t.Fatalf("err %v, posted %+v", err, posted)
		}
		if got.TransactionHash.Hex() != posted[1].hash || len(stored) != 2 || stored[1].Replaces.Hex() != posted[0].hash {
			t.Fatalf("submission %+v, stored %+v", got, stored)
		}
	}

	node := newMockNode(t)
	node.queuePost(authorityRoute, notCommitted, notCommitted)
	_, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeSettled, legacyPause(11), testSigner(t))
	if !isRefusal(err, RefusalReferenceCheckpointNotCommitted) || len(node.postedRequests()) != 2 {
		t.Fatalf("second refusal: err %v, posted %d", err, len(node.postedRequests()))
	}
}

func TestOtherRefusalsOfTheSettledEnvelopeAreNotRetried(t *testing.T) {
	referenceRetryDelayForTest(t)
	for _, kind := range []string{"reference_checkpoint_too_old", "settled_envelope_not_active"} {
		node := newMockNode(t)
		node.queuePost(authorityRoute, problemReply(http.StatusUnprocessableEntity, kind, "retry_later", nil))
		_, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeSettled, legacyPause(11), testSigner(t))
		if refusal, ok := AuthorityRefusalOf(err); !ok || string(refusal.Kind) != kind || len(node.postedRequests()) != 1 {
			t.Fatalf("%s: err %v, posted %d", kind, err, len(node.postedRequests()))
		}
	}
}

func TestAStoreThatFailsSendsNothing(t *testing.T) {
	node := newMockNode(t)
	storeFailed := errors.New("disk full")
	hook := WithOnSigned(func(context.Context, SignedTransaction) error { return storeFailed })
	_, err := node.client().Tokens().SendAuthorityOperation(context.Background(), AuthorityEnvelopeSettled, legacyPause(11), testSigner(t), hook)
	if !errors.Is(err, storeFailed) || len(node.postedRequests()) != 0 {
		t.Fatalf("err %v, posted %d", err, len(node.postedRequests()))
	}
}

func TestASettledListOperationNamesTheListOfTheTokensPrivacy(t *testing.T) {
	node := newMockNode(t)
	node.private = true
	freeze := LegacyBlacklist(TokenManageListPayload{ChainID: 1212101, Nonce: 3, Action: ManageListActionAdd, Address: repeatAddr(2), Token: repeatAddr(1)})
	if _, err := node.client().Tokens().AuthorityOperationByMode(context.Background(), freeze, testSigner(t)); err != nil {
		t.Fatal(err)
	}
	operation := node.postedRequests()[0].body["operation"].(map[string]any)
	if operation["list"] != "whitelist" || operation["action"] != "remove" {
		t.Fatalf("operation = %v", operation)
	}
}

func TestALegacyV1ClientSendsTheSettledEnvelopeOnV2(t *testing.T) {
	node := newMockNode(t)
	got, err := node.client(WithLegacyV1()).Tokens().AuthorityOperationByMode(context.Background(), legacyPause(11), testSigner(t))
	if err != nil || got.Envelope != AuthorityEnvelopeSettled || node.postedRequests()[0].path != authorityRoute {
		t.Fatalf("submission %+v, err %v", got, err)
	}
	_, err = node.client(WithLegacyV1()).Tokens().Pause(context.Background(), PauseTokenPayload{ChainID: 1, Nonce: 1, Token: repeatAddr(1)}, testSigner(t), WithOnSigned(func(context.Context, SignedTransaction) error { return nil }))
	if err == nil || !strings.Contains(err.Error(), "WithOnSigned") {
		t.Fatalf("a legacy v1 send cannot hand over its transaction: %v", err)
	}
}

func TestLatestReferenceCheckpointAndMemo(t *testing.T) {
	node := newMockNode(t)
	node.setCheckpoints(77)
	if n, err := node.client().LatestReferenceCheckpoint(context.Background()); err != nil || n != 77 {
		t.Fatalf("n %d err %v", n, err)
	}
	memo := Memo{Type: "case", Format: "text", Data: "7"}
	if _, err := node.client().Tokens().AuthorityOperationByMode(context.Background(), legacyPause(1), testSigner(t), WithMemo(memo)); err != nil {
		t.Fatal(err)
	}
	if got := node.postedRequests()[0].body["memo"].(map[string]any); got["data"] != "7" {
		t.Fatalf("memo = %v", got)
	}
}

// Every transaction that can reach OnSigned has a stored form, so a hook never
// receives a transaction that it cannot store.
func TestEveryTransactionHandedToOnSignedIsStorable(t *testing.T) {
	referenceRetryDelayForTest(t)
	token := repeatAddr(0x01)
	account := repeatAddr(0x02)
	list := TokenManageListPayload{ChainID: 1212101, Nonce: 3, Action: ManageListActionAdd, Address: account, Token: token}
	operations := []LegacyAuthorityOperation{
		LegacyMint(TokenMintPayload{ChainID: 1212101, Nonce: 3, Recipient: account, Value: big.NewInt(5), Token: token}),
		LegacyAuthority(TokenAuthorityPayload{ChainID: 1212101, Nonce: 3, Action: AuthorityActionRevoke, AuthorityType: AuthorityTypeClawback, AuthorityAddress: account, Token: token, Value: big.NewInt(0)}),
		LegacyBlacklist(list), LegacyWhitelist(list),
		legacyPause(3),
		LegacyMetadata(UpdateMetadataPayload{ChainID: 1212101, Nonce: 3, Name: "N", URI: "u", Token: token}),
	}
	var handed []SignedTransaction
	for _, operation := range operations {
		for _, envelope := range []AuthorityEnvelope{AuthorityEnvelopeLegacy, AuthorityEnvelopeSettled} {
			node := newMockNode(t)
			hook := WithOnSigned(func(_ context.Context, signed SignedTransaction) error {
				handed = append(handed, signed)
				return nil
			})
			if _, err := node.client().Tokens().SendAuthorityOperation(context.Background(), envelope, operation, testSigner(t), hook); err != nil {
				t.Fatalf("%+v in %s: %v", operation, envelope, err)
			}
		}
	}
	clawback := TokenClawbackPayload{ChainID: 1212101, Nonce: 3, Token: token, From: account, Recipient: repeatAddr(3), Value: big.NewInt(4)}
	verifyClawback := clawback
	verifyClawback.Token = VerifyTokenAddress
	for _, payload := range []any{clawback, verifyClawback, laneVectorPayload()} {
		node := newMockNode(t)
		hook := WithOnSigned(func(_ context.Context, signed SignedTransaction) error {
			handed = append(handed, signed)
			return nil
		})
		if err := node.client().submitPayload(context.Background(), payload, resolveSubmit([]SubmitOption{hook}), testSigner(t), new(TransactionResponse)); err != nil {
			t.Fatalf("%T: %v", payload, err)
		}
	}
	if len(handed) != 2*len(operations)+3 {
		t.Fatalf("handed %d transactions", len(handed))
	}
	for _, signed := range handed {
		stored, err := json.Marshal(signed.Transaction)
		if err != nil {
			t.Fatalf("%s: %v", signed.Transaction.path, err)
		}
		if _, err := ParseAuthorizedTransaction(stored); err != nil {
			t.Fatalf("%s: %v", signed.Transaction.path, err)
		}
	}
}

func TestWithOnSignedRefusesATransactionWithoutAStoredForm(t *testing.T) {
	node := newMockNode(t)
	called := false
	hook := WithOnSigned(func(context.Context, SignedTransaction) error { called = true; return nil })
	_, err := node.client().Transactions().Payment(context.Background(), testPaymentPayload(), testSigner(t), hook)
	if err == nil || !strings.Contains(err.Error(), "authority-class") || called || len(node.postedRequests()) != 0 {
		t.Fatalf("err %v, hook called %t, posted %d", err, called, len(node.postedRequests()))
	}
}
