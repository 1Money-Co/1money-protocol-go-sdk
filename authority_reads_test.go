package onemoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

const someHash = "0x1111111111111111111111111111111111111111111111111111111111111111"

func TestGetTransactionStatus(t *testing.T) {
	node := newMockNode(t)
	node.setTxStatus(someHash, okJSON(`{"hash":"`+someHash+`","status":"certified_pending_checkpoint"}`))
	status, err := node.client().GetTransactionStatus(context.Background(), someHash)
	if err != nil || status.Status != TransactionStatusCertifiedPendingCheckpoint || status.CheckpointNumber != nil {
		t.Fatalf("status = %+v, err = %v", status, err)
	}
	for value, counterSigned := range map[TransactionStatus]bool{
		TransactionStatusPending: false, TransactionStatusFinalized: true,
		TransactionStatusCertifiedPendingCheckpoint: true, TransactionStatusCheckpointed: true,
		"evicted": false,
	} {
		if value.IsCounterSigned() != counterSigned {
			t.Errorf("%s.IsCounterSigned() = %t", value, !counterSigned)
		}
	}
	if TransactionStatus("evicted").IsKnown() || !TransactionStatusPending.IsKnown() {
		t.Error("IsKnown")
	}
}

func receiptFrom(t *testing.T, body string) *TransactionReceiptResponse {
	t.Helper()
	var receipt TransactionReceiptResponse
	if err := json.Unmarshal([]byte(body), &receipt); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return &receipt
}

func TestReceiptOutcomeOf(t *testing.T) {
	target := common.HexToAddress("0x9E1E9688A44D058fF181Ed64ddFAFbBE5CC742Ab")
	success := receiptFrom(t, `{"success":true,"transaction_hash":"0x1","success_info":{"message":"ok"}}`)
	if got := ReceiptOutcomeOf(success); got.Kind != ReceiptSuccess {
		t.Fatalf("success: %+v", got)
	}

	noOp := receiptFrom(t, `{"success":true,"execution_events":[{"event_type":"NoOp","target":null}]}`)
	if got := ReceiptOutcomeOf(noOp); got.Kind != ReceiptNoOp || got.Target != nil || got.NoOpRevoke != nil {
		t.Fatalf("no-op: %+v", got)
	}

	revoke := receiptFrom(t, `{"success":true,"execution_events":[{"event_type":"NoOp","target":"`+target.Hex()+`"}],"warnings":[{"type":"no_op_revoke","target":"`+target.Hex()+`"}]}`)
	got := ReceiptOutcomeOf(revoke)
	if got.Kind != ReceiptNoOp || got.Target == nil || *got.Target != target || got.NoOpRevoke == nil || *got.NoOpRevoke.Target != target {
		t.Fatalf("no-op revoke: %+v", got)
	}

	reasons := []SoftFailReason{
		SoftFailHoldNotFound, SoftFailHoldInsufficient, SoftFailComplianceRejected, SoftFailAuthorityChanged,
		SoftFailRoleMissing, SoftFailCapReached, SoftFailAllowanceShort, SoftFailRotationConflict, SoftFailRecipientNotWallet,
	}
	for _, reason := range reasons {
		receipt := receiptFrom(t, fmt.Sprintf(`{"success":false,"execution_events":[{"event_type":"SoftFail","reason":%q}]}`, reason))
		if got := ReceiptOutcomeOf(receipt); got.Kind != ReceiptSoftFail || got.SoftFailReason != reason || !reason.IsKnown() {
			t.Fatalf("%s: %+v", reason, got)
		}
	}

	unknown := receiptFrom(t, `{"success":false,"execution_events":[{"event_type":"SoftFail","reason":"MoonPhase"}]}`)
	if got := ReceiptOutcomeOf(unknown); got.Kind != ReceiptSoftFail || got.SoftFailReason != "MoonPhase" || got.SoftFailReason.IsKnown() {
		t.Fatalf("unknown reason: %+v", got)
	}

	failed := receiptFrom(t, `{"success":false}`)
	if got := ReceiptOutcomeOf(failed); got.Kind != ReceiptFailed {
		t.Fatalf("failed: %+v", got)
	}

	newEvent := receiptFrom(t, `{"success":true,"execution_events":[{"event_type":"Sparkle","glitter":1}]}`)
	if got := ReceiptOutcomeOf(newEvent); got.Kind != ReceiptSuccess {
		t.Fatalf("an unknown event type is not a no-op: %+v", got)
	}
}

func TestAuthorityRefusalOf(t *testing.T) {
	members := map[string]map[string]any{
		"reference_checkpoint_not_committed": {"checkpoint_id": 12, "checkpoint_id_latest": 10},
		"reference_checkpoint_too_old":       {"checkpoint_id": 1, "checkpoint_id_earliest": 400},
		"token_not_at_reference_checkpoint":  {"mint_address": "0x0101010101010101010101010101010101010101", "checkpoint_id": 12},
		"legacy_envelope_retired":            nil,
		"settled_envelope_not_active":        nil,
	}
	for kind, extra := range members {
		node := newMockNode(t)
		node.queuePost("/v2/tokens/authority_operation", problemReply(http.StatusUnprocessableEntity, kind, "retry_later", extra))
		_, err := node.client().Submit(context.Background(), mustAuthorize(t, settledVectorPayload(SettledPause{})))
		refusal, ok := AuthorityRefusalOf(err)
		if !ok || string(refusal.Kind) != kind || refusal.Err == nil {
			t.Fatalf("%s: refusal = %+v, ok = %t (err %v)", kind, refusal, ok, err)
		}
		switch kind {
		case "reference_checkpoint_not_committed":
			if *refusal.CheckpointID != 12 || *refusal.CheckpointIDLatest != 10 {
				t.Fatalf("%s: %+v", kind, refusal)
			}
		case "reference_checkpoint_too_old":
			if *refusal.CheckpointIDEarliest != 400 {
				t.Fatalf("%s: %+v", kind, refusal)
			}
		case "token_not_at_reference_checkpoint":
			if *refusal.MintAddress != repeatAddr(0x01) {
				t.Fatalf("%s: %+v", kind, refusal)
			}
		}
	}

	for _, err := range []error{
		nil,
		fmt.Errorf("transport"),
		&APIError{StatusCode: 422, Problem: Problem{Type: ProblemTypePrefix + "nonce_too_low"}},
		&APIError{StatusCode: 500, Problem: Problem{Type: ProblemTypePrefix + "legacy_envelope_retired"}},
		&APIError{StatusCode: 422, Problem: Problem{Type: "about:blank"}},
	} {
		if refusal, ok := AuthorityRefusalOf(err); ok {
			t.Errorf("%v read as the refusal %+v", err, refusal)
		}
	}
	wrapped := fmt.Errorf("send: %w", &APIError{StatusCode: 422, Problem: Problem{Type: ProblemTypePrefix + "legacy_envelope_retired"}})
	if refusal, ok := AuthorityRefusalOf(wrapped); !ok || refusal.Kind != RefusalLegacyEnvelopeRetired {
		t.Errorf("a wrapped refusal: %+v %t", refusal, ok)
	}
}

func TestTheProblemCatalogKnowsTheTokenAuthorityTypes(t *testing.T) {
	for _, id := range []string{
		"legacy_envelope_retired", "settled_envelope_not_active", "reference_checkpoint_not_committed",
		"reference_checkpoint_too_old", "token_not_at_reference_checkpoint", "lane_order_pending",
		"signer_account_not_committed", "governance_proposal_conflict", "withdrawal_exceeds_allowance",
		"withdrawal_rate_limited",
	} {
		if !(Problem{Type: ProblemTypePrefix + id}).IsKnownType() {
			t.Errorf("%s is not in the catalog", id)
		}
	}
}

func mustAuthorize(t *testing.T, payload any, opts ...SubmitOption) *AuthorizedTransaction {
	t.Helper()
	authorized, _ := authorizedBody(t, payload, opts...)
	return authorized
}

func TestTransactionViewOfTheNewTypes(t *testing.T) {
	var settled Transaction
	if err := json.Unmarshal([]byte(`{"hash":"0x1","transaction_type":"TokenAuthorityOperation","data":{"token":"0x0101010101010101010101010101010101010101","reference_checkpoint":42,"operation":{"type":"revoke_role","role":"clawback","account":"0x0202020202020202020202020202020202020202"}}}`), &settled); err != nil {
		t.Fatal(err)
	}
	data, ok := settled.AsTokenAuthorityOperationData()
	if !ok || data.ReferenceCheckpoint != 42 || !strings.Contains(string(data.Operation), "revoke_role") {
		t.Fatalf("settled view: %+v %t", data, ok)
	}
	var lane Transaction
	if err := json.Unmarshal([]byte(`{"hash":"0x2","nonce":4,"lane":"0x0202020202020202020202020202020202020202","transaction_type":"TokenLaneClawback","data":{"token":"0x0101010101010101010101010101010101010101","holder":"0x0202020202020202020202020202020202020202","from_nonce":4,"reference_checkpoint":42,"recipient":"0x0303030303030303030303030303030303030303","value":"500"}}`), &lane); err != nil {
		t.Fatal(err)
	}
	laneData, ok := lane.AsTokenLaneClawbackData()
	if !ok || laneData.FromNonce != 4 || laneData.Value != "500" || lane.Lane == nil || *lane.Lane != repeatAddr(0x02) {
		t.Fatalf("lane view: %+v %t lane=%v", laneData, ok, lane.Lane)
	}
}
