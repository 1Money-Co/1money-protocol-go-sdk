package onemoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ourHash     = common.HexToHash("0x" + strings.Repeat("11", 32))
	anotherHash = common.HexToHash("0x" + strings.Repeat("22", 32))
)

const (
	nonceTooLowType = ProblemTypePrefix + "nonce_too_low"
	inFlightType    = ProblemTypePrefix + "submission_already_in_flight"
	newerNodeType   = ProblemTypePrefix + "future_kind"
)

func hexOf(hash common.Hash) string { return hash.Hex() }

func httpAnswer(status int, body string) answer {
	return answer{status: status, body: []byte(body)}
}

func jsonBody(t testing.TB, members map[string]any) string {
	t.Helper()
	body, err := json.Marshal(members)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func classifyKind(status int, body string) submitOutcomeKind {
	return classifySubmit(httpAnswer(status, body), ourHash).kind
}

func classifyJSON(t *testing.T, status int, members map[string]any) submitOutcomeKind {
	t.Helper()
	return classifyKind(status, jsonBody(t, members))
}

func TestRule1A2xxWithAStringHashIsAcceptedWithTheServerHash(t *testing.T) {
	for _, status := range []int{200, 201, 299} {
		body := jsonBody(t, map[string]any{"hash": hexOf(anotherHash)})
		outcome := classifySubmit(httpAnswer(status, body), ourHash)
		if outcome.kind != outcomeAccepted || outcome.status != status || string(outcome.body) != body || outcome.serverHash != hexOf(anotherHash) {
			t.Fatalf("status %d: %+v", status, outcome)
		}
	}
}

func TestRule1WinsOverEveryAction(t *testing.T) {
	for _, action := range []string{"fix_request", "retry_later", "use_other_node", "in_progress", "check_status"} {
		for _, status := range []int{200, 201, 299} {
			body := map[string]any{"hash": hexOf(ourHash), "type": newerNodeType, "action": action, "tx_hash": hexOf(anotherHash)}
			if got := classifyJSON(t, status, body); got != outcomeAccepted {
				t.Errorf("%s at %d: %v", action, status, got)
			}
		}
	}
}

func TestA2xxWithoutAStringHashIsUnknown(t *testing.T) {
	for _, body := range []string{`{}`, `{"hash":42}`, `{"hash":null}`, `[{"hash":"` + hexOf(ourHash) + `"}]`, ``, `ok`} {
		if got := classifyKind(200, body); got != outcomeUnknown {
			t.Errorf("%s: %v", body, got)
		}
	}
}

func TestAHashOutsideA2xxDoesNotAccept(t *testing.T) {
	for _, status := range []int{199, 300, 400, 500} {
		if got := classifyJSON(t, status, map[string]any{"hash": hexOf(ourHash), "error_code": "x"}); got != outcomeUnknown {
			t.Errorf("%d: %v", status, got)
		}
	}
}

func TestRule2ANoEffectActionIsRejectedAtAnyStatus(t *testing.T) {
	for _, action := range []string{"fix_request", "retry_later", "use_other_node"} {
		for _, status := range []int{200, 400, 408, 422, 429, 500, 503} {
			body := map[string]any{"type": newerNodeType, "action": action, "error_code": "business_transaction_failed"}
			if got := classifyJSON(t, status, body); got != outcomeRejected {
				t.Errorf("%s at %d: %v", action, status, got)
			}
		}
	}
}

func TestRule2NodeSyncingAndSubmitQueueFullAreRejected(t *testing.T) {
	if got := classifyJSON(t, 503, map[string]any{"type": ProblemTypePrefix + "node_syncing", "action": "retry_later"}); got != outcomeRejected {
		t.Errorf("node_syncing: %v", got)
	}
	if got := classifyJSON(t, 500, map[string]any{"type": ProblemTypePrefix + "submit_queue_full", "action": "retry_later"}); got != outcomeRejected {
		t.Errorf("submit_queue_full: %v", got)
	}
}

func TestRule3InProgressForThisTransactionIsLocked(t *testing.T) {
	upper := "0x" + strings.ToUpper(hexOf(ourHash)[2:])
	for _, tc := range []struct {
		status int
		body   map[string]any
	}{
		{422, map[string]any{"type": inFlightType, "action": "in_progress"}},
		{422, map[string]any{"type": inFlightType, "action": "in_progress", "tx_hash": hexOf(ourHash)}},
		{422, map[string]any{"type": inFlightType, "action": "in_progress", "tx_hash": upper}},
		{500, map[string]any{"type": inFlightType, "action": "in_progress"}},
	} {
		if got := classifyJSON(t, tc.status, tc.body); got != outcomeLocked {
			t.Errorf("%d %v: %v", tc.status, tc.body, got)
		}
	}
}

func TestRule4InProgressForAnotherOrUnreadableHashIsUnknown(t *testing.T) {
	for _, txHash := range []any{hexOf(anotherHash), 42, nil, "0xnothex", "", []string{hexOf(ourHash)}} {
		body := map[string]any{"type": inFlightType, "action": "in_progress", "tx_hash": txHash}
		if got := classifyJSON(t, 422, body); got != outcomeUnknown {
			t.Errorf("tx_hash %v: %v", txHash, got)
		}
	}
}

func TestRule4AnyOtherActionIsUnknownAndKeepsTheRawAction(t *testing.T) {
	for _, action := range []string{`"check_status"`, `"IN_PROGRESS"`, `"Fix_Request"`, `"later"`, `null`, `7`} {
		body := `{"type":"about:blank","title":"t","action":` + action + `,"error_code":"validation_x"}`
		outcome := classifySubmit(httpAnswer(400, body), ourHash)
		if outcome.kind != outcomeUnknown || outcome.detail.apiErr == nil {
			t.Fatalf("%s: %+v", action, outcome)
		}
		if string(outcome.detail.apiErr.RawAction) != action || string(outcome.detail.action) != action {
			t.Errorf("%s: raw %s, node action %s", action, outcome.detail.apiErr.RawAction, outcome.detail.action)
		}
	}
}

func legacyMessageBody(t *testing.T, message string) map[string]any {
	t.Helper()
	return map[string]any{"error_code": "business_transaction_failed", "message": message}
}

func pinned(reason string) string { return "Transaction processing failed: " + reason }

func unknownNonceUsed(t *testing.T, a answer) bool {
	t.Helper()
	outcome := classifySubmit(a, ourHash)
	if outcome.kind != outcomeUnknown {
		t.Fatalf("expected unknown, got %+v", outcome)
	}
	return outcome.nonceUsed
}

func refusalOf(t *testing.T, status int, members map[string]any) SubmitRefusal {
	t.Helper()
	outcome := classifySubmit(httpAnswer(status, jsonBody(t, members)), ourHash)
	if outcome.kind != outcomeRejected {
		t.Fatalf("expected rejected, got %+v", outcome)
	}
	return outcome.refusal
}

func TestRule5Legacy422BusinessTransactionFailedIsUnknownForAnyMessage(t *testing.T) {
	cases := []struct {
		status int
		body   map[string]any
	}{
		{422, legacyMessageBody(t, "already inflight; please retry later")},
		{422, legacyMessageBody(t, pinned("Insufficient balance: required 10, available 1"))},
		{422, map[string]any{"error_code": "business_insufficient_funds", "message": "m"}},
		{400, legacyMessageBody(t, "already inflight; please retry later")},
		{422, legacyMessageBody(t, pinned("already inflight; please retry later"))},
		{400, legacyMessageBody(t, pinned("already inflight; please retry later"))},
		{422, legacyMessageBody(t, pinned("Failed to recover REST envelope: invalid signature bytes"))},
		{422, legacyMessageBody(t, pinned("Invalid chain id: expected 1212101, actual 1"))},
		{422, legacyMessageBody(t, pinned("Invalid transaction memo: too long"))},
		{422, map[string]any{"type": "about:blank", "action": "check_status", "error_code": "business_transaction_failed", "message": pinned("already inflight; please retry later")}},
	}
	for _, tc := range cases {
		if got := classifyJSON(t, tc.status, tc.body); got != outcomeUnknown {
			t.Errorf("%d %v: %v", tc.status, tc.body, got)
		}
	}
	for _, reason := range []string{"nonce already used; state advanced", "Execution admission denied: nonce too low to execute: expected 4, got 3 of tx 0xab"} {
		if unknownNonceUsed(t, httpAnswer(422, jsonBody(t, legacyMessageBody(t, pinned(reason))))) {
			t.Errorf("%s marks the nonce used", reason)
		}
	}
}

func TestTheNonceTooLowTypeMarksTheNonceUsed(t *testing.T) {
	typed := func(typeURI string) string {
		return jsonBody(t, map[string]any{"type": typeURI, "title": "Nonce too low", "action": "check_status", "error_code": "business_transaction_failed"})
	}
	if !unknownNonceUsed(t, httpAnswer(422, typed(nonceTooLowType))) {
		t.Error("422 nonce_too_low")
	}
	if unknownNonceUsed(t, httpAnswer(422, typed(nonceTooLowType+"x"))) {
		t.Error("a longer type is not nonce_too_low")
	}
	if !unknownNonceUsed(t, httpAnswer(500, typed(nonceTooLowType))) {
		t.Error("500 nonce_too_low")
	}
}

func TestOnlyTheBytesRefusalKindsRefuseTheBytes(t *testing.T) {
	for _, id := range []string{"envelope_signature_invalid", "signature_pairing_invalid", "chain_id_mismatch", "sender_not_allowed", "memo_invalid"} {
		if got := refusalOf(t, 422, map[string]any{"type": ProblemTypePrefix + id, "title": "t", "action": "fix_request"}); got != SubmitRefusalBytes {
			t.Errorf("%s: %v", id, got)
		}
	}
	for _, id := range []string{"transaction_too_large", "batch_payment_invalid", "bridge_sender_mismatch", "provenance_not_admissible", "compliance_blocked", "withdrawal_exceeds_allowance"} {
		if got := refusalOf(t, 422, map[string]any{"type": ProblemTypePrefix + id, "title": "t", "action": "fix_request"}); got != SubmitRefusalRequest {
			t.Errorf("%s: %v", id, got)
		}
	}
	if got := refusalOf(t, 400, map[string]any{"type": "about:blank", "action": "fix_request", "error_code": "x", "message": "m"}); got != SubmitRefusalRequest {
		t.Errorf("about:blank: %v", got)
	}
}

func TestRule6A408IsUnknown(t *testing.T) {
	if got := classifyJSON(t, 408, map[string]any{"error_code": "request_timeout", "message": "m"}); got != outcomeUnknown {
		t.Errorf("json: %v", got)
	}
	if got := classifyKind(408, "Request Timeout"); got != outcomeUnknown {
		t.Errorf("text: %v", got)
	}
}

func TestA4xxWithoutAnActionIsUnknown(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 405, 407, 409, 410, 413, 421, 423, 429, 499} {
		if got := classifyJSON(t, status, map[string]any{"error_code": "c", "message": "m"}); got != outcomeUnknown {
			t.Errorf("%d: %v", status, got)
		}
	}
	if got := classifyKind(403, "<html><body>403 Forbidden</body></html>"); got != outcomeUnknown {
		t.Errorf("html 403: %v", got)
	}
	if got := classifyKind(429, "Too Many Requests"); got != outcomeUnknown {
		t.Errorf("text 429: %v", got)
	}
}

func TestAJSONBodyThatIsNotAnObjectHasNoFields(t *testing.T) {
	for _, body := range []string{`[{"action":"in_progress"}]`, `null`, `"in_progress"`, `1`} {
		for _, status := range []int{422, 200} {
			if got := classifyKind(status, body); got != outcomeUnknown {
				t.Errorf("%d %s: %v", status, body, got)
			}
		}
	}
}

func TestATxHashWithout0xStillNamesTheLocalHash(t *testing.T) {
	bare := hexOf(ourHash)[2:]
	if got := classifyJSON(t, 422, map[string]any{"type": inFlightType, "action": "in_progress", "tx_hash": bare}); got != outcomeLocked {
		t.Errorf("%v", got)
	}
}

func TestA4xxBodyThatLooksLikeAnObjectButDoesNotParseIsUnknown(t *testing.T) {
	deep := `{"action":"in_progress","x":` + strings.Repeat("[", 200) + strings.Repeat("]", 200) + `}`
	for _, body := range []string{deep, `{"action":"in_progress","tx_hash":"0x11`, `{"n":1e400}`, `{"m":"\ud800"}`, `  {`} {
		if got := classifyKind(422, body); got != outcomeUnknown {
			t.Errorf("%s: %v", body, got)
		}
	}
}

func TestARedirectedAnswerIsUnknown(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{404, `{"error_code":"resource_not_found","message":"m"}`},
		{400, `{"type":"about:blank","action":"fix_request"}`},
		{200, `{"hash":"` + hexOf(ourHash) + `"}`},
	} {
		outcome := classifySubmit(answer{status: tc.status, body: []byte(tc.body), redirected: true}, ourHash)
		if outcome.kind != outcomeUnknown || !strings.HasPrefix(outcome.detail.summary.Message, "the answer came after an HTTP redirect: ") {
			t.Errorf("%d %s: %+v", tc.status, tc.body, outcome)
		}
	}
}

func TestA4xxWithAnEmptyBodyIsUnknown(t *testing.T) {
	if got := classifyKind(400, ""); got != outcomeUnknown {
		t.Errorf("400: %v", got)
	}
	if got := classifyKind(429, " \n\t"); got != outcomeUnknown {
		t.Errorf("429: %v", got)
	}
}

func TestRule8OtherStatusesAndTransportAreUnknown(t *testing.T) {
	for _, status := range []int{100, 199, 204, 300, 302, 399, 500, 502, 503, 504, 599} {
		if got := classifyJSON(t, status, map[string]any{"error_code": "system_x", "message": "m"}); got != outcomeUnknown {
			t.Errorf("%d: %v", status, got)
		}
	}
	for _, status := range []int{0, 200} {
		if got := classifySubmit(answer{status: status, err: errors.New("broken")}, ourHash).kind; got != outcomeUnknown {
			t.Errorf("transport with status %d: %v", status, got)
		}
	}
}

func TestSummaryTakesTheMessageThenTheTitleThenTheBody(t *testing.T) {
	legacy := classifySubmit(httpAnswer(422, `{"error_code":"business_transaction_failed","message":"already inflight","title":"t"}`), ourHash).detail.summary
	if legacy.Status != 422 || legacy.ErrorCode != "business_transaction_failed" || legacy.Message != "already inflight" {
		t.Errorf("legacy: %+v", legacy)
	}
	if got := classifySubmit(httpAnswer(429, `{"type":"about:blank","title":"Rate limited","action":"check_status"}`), ourHash).detail.summary.Message; got != "Rate limited" {
		t.Errorf("title: %q", got)
	}
	long := strings.Repeat("x", 500)
	if got := classifySubmit(httpAnswer(403, "  "+long+"  "), ourHash).detail.summary.Message; got != strings.Repeat("x", 200) {
		t.Errorf("body: %q", got)
	}
	if got := classifySubmit(httpAnswer(502, ""), ourHash).detail.summary.Message; got != "empty response body" {
		t.Errorf("empty: %q", got)
	}
}

func TestSummaryOfATransportFailureNamesItsKind(t *testing.T) {
	summary := classifySubmit(answer{err: errors.New("broken")}, ourHash).detail.summary
	if summary.Status != 0 || !strings.HasPrefix(summary.Message, "request failed: ") {
		t.Errorf("no answer: %+v", summary)
	}
	bodyRead := classifySubmit(answer{status: 200, err: errors.New("cut")}, ourHash).detail.summary
	if bodyRead.Status != 200 || !strings.HasPrefix(bodyRead.Message, "body read failed: ") {
		t.Errorf("body read: %+v", bodyRead)
	}
}

func TestARefusedConnectionSaysConnectionFailed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	client := NewClientWithCustomUrl("http://" + address)
	got, err := client.postAnswer(context.Background(), "/v2/transactions/payment", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if message := classifySubmit(got, ourHash).detail.summary.Message; !strings.HasPrefix(message, "connection failed: ") {
		t.Errorf("%q", message)
	}
}

func TestASilentServerSaysRequestTimedOut(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	client := NewClientWithCustomUrl("http://"+listener.Addr().String(), WithTimeout(100*time.Millisecond))
	got, err := client.postAnswer(context.Background(), "/v2/transactions/payment", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if message := classifySubmit(got, ourHash).detail.summary.Message; !strings.HasPrefix(message, "request timed out: ") {
		t.Errorf("%q", message)
	}
}

func TestAPostThatARedirectTurnsIntoAGetIsRedirected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/transactions/payment", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusSeeOther)
	})
	mux.HandleFunc("/elsewhere", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"hash":%q}`, hexOf(ourHash))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	got, err := NewClientWithCustomUrl(server.URL).postAnswer(context.Background(), "/v2/transactions/payment", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.redirected || classifySubmit(got, ourHash).kind != outcomeUnknown {
		t.Fatalf("%+v", got)
	}
}

func TestTheProblemIsAttachedWhenTheBodyHasOne(t *testing.T) {
	body := jsonBody(t, map[string]any{"type": inFlightType, "title": "Transaction already in flight", "status": 422, "action": "in_progress", "tx_hash": hexOf(ourHash)})
	outcome := classifySubmit(httpAnswer(422, body), ourHash)
	if outcome.kind != outcomeLocked || outcome.detail.apiErr == nil || outcome.detail.apiErr.Type != inFlightType {
		t.Fatalf("%+v", outcome)
	}
	if hash, ok := outcome.detail.apiErr.TxHash(); !ok || hash != hexOf(ourHash) {
		t.Errorf("tx_hash %q %t", hash, ok)
	}
}

func TestARejectionKeepsTheProblem(t *testing.T) {
	body := jsonBody(t, map[string]any{"type": ProblemTypePrefix + "compliance_blocked", "title": "Blocked by compliance", "status": 403, "action": "fix_request"})
	outcome := classifySubmit(httpAnswer(403, body), ourHash)
	if outcome.kind != outcomeRejected || outcome.refusal != SubmitRefusalRequest || outcome.detail.summary.Status != 403 {
		t.Fatalf("%+v", outcome)
	}
	if outcome.detail.apiErr.Action != ActionFixRequest {
		t.Errorf("action %q", outcome.detail.apiErr.Action)
	}
}

func receiptBody(hash common.Hash) string {
	return `{"success":true,"transaction_hash":"` + hexOf(hash) + `","transaction_index":null,"checkpoint_hash":null,"checkpoint_number":null,"from":"0x0000000000000000000000000000000000000000","fee_used":"0","recipient":null,"token_address":null}`
}

func unreadableReceiptBody(hash common.Hash) string {
	return strings.Replace(receiptBody(hash), `"fee_used":"0"`, `"fee_used":{"shape":"of a newer node"}`, 1)
}

func TestReadReceiptFindsOnlyA2xxReceiptForTheHash(t *testing.T) {
	if got := readReceipt(httpAnswer(200, receiptBody(ourHash)), ourHash); got.kind != readFound || got.value.TransactionHash != hexOf(ourHash) {
		t.Fatalf("%+v", got)
	}
	for _, a := range []answer{httpAnswer(200, receiptBody(anotherHash)), httpAnswer(200, "<html>proxy</html>"), httpAnswer(500, receiptBody(ourHash))} {
		if got := readReceipt(a, ourHash); got.kind != readFailed {
			t.Errorf("%d %s: %v", a.status, a.body, got.kind)
		}
	}
}

func TestA2xxReceiptForTheHashThatDoesNotDecodeIsUnreadable(t *testing.T) {
	if got := readReceipt(httpAnswer(200, unreadableReceiptBody(ourHash)), ourHash).kind; got != readUnreadable {
		t.Errorf("for the hash: %v", got)
	}
	if got := readReceipt(httpAnswer(200, unreadableReceiptBody(anotherHash)), ourHash).kind; got != readFailed {
		t.Errorf("for another hash: %v", got)
	}
	if got := readReceipt(httpAnswer(500, unreadableReceiptBody(ourHash)), ourHash).kind; got != readFailed {
		t.Errorf("500: %v", got)
	}
}

func TestReadReceiptMapsNotFoundThrottledAndFailed(t *testing.T) {
	notFound := `{"error_code":"resource_transaction_not_found","message":"m"}`
	if got := readReceipt(httpAnswer(404, notFound), ourHash); got.kind != readNotFound || got.detail.summary.ErrorCode != "resource_transaction_not_found" {
		t.Errorf("404: %+v", got)
	}
	if got := readReceipt(httpAnswer(429, ""), ourHash); got.kind != readThrottled || got.detail.summary.Status != 429 {
		t.Errorf("429: %+v", got)
	}
	retryLater := `{"type":"` + newerNodeType + `","action":"retry_later"}`
	for _, a := range []answer{httpAnswer(503, retryLater), httpAnswer(404, retryLater)} {
		if got := readReceipt(a, ourHash).kind; got != readThrottled {
			t.Errorf("%d retry_later: %v", a.status, got)
		}
	}
	for _, a := range []answer{httpAnswer(502, "bad gateway"), httpAnswer(400, notFound), {err: errors.New("down")}} {
		if got := readReceipt(a, ourHash).kind; got != readFailed {
			t.Errorf("%d: %v", a.status, got)
		}
	}
}

func TestReadTransactionNeedsOnlyTheHash(t *testing.T) {
	if got := readTransaction(httpAnswer(200, `{"hash":"`+hexOf(ourHash)+`","a_member_this_sdk_does_not_know":1}`), ourHash).kind; got != readFound {
		t.Errorf("found: %v", got)
	}
	if got := readTransaction(httpAnswer(200, `{"hash":"`+hexOf(anotherHash)+`"}`), ourHash).kind; got != readFailed {
		t.Errorf("other: %v", got)
	}
	if got := readTransaction(httpAnswer(404, `{}`), ourHash).kind; got != readNotFound {
		t.Errorf("404: %v", got)
	}
	if got := readTransaction(httpAnswer(429, `{}`), ourHash).kind; got != readThrottled {
		t.Errorf("429: %v", got)
	}
}

func TestAFailedReadKeepsTheAnswerSummary(t *testing.T) {
	got := readReceipt(httpAnswer(500, `{"error_code":"system_x","message":"db down"}`), ourHash)
	if got.kind != readFailed || got.detail.summary != (AnswerSummary{Status: 500, ErrorCode: "system_x", Message: "db down"}) {
		t.Errorf("%+v", got)
	}
}

// --- Properties ---

type propertyBody struct {
	hash, typeURI, action, txHash, errorCode *json.RawMessage
	message                                  string
}

func raw(v any) *json.RawMessage {
	b, _ := json.Marshal(v)
	r := json.RawMessage(b)
	return &r
}

func randomString(r *rand.Rand) string {
	alphabet := []rune("abc_xyz0123456789 /:.é世\"\\")
	out := make([]rune, r.IntN(12))
	for i := range out {
		out[i] = alphabet[r.IntN(len(alphabet))]
	}
	return string(out)
}

func pick[T any](r *rand.Rand, values ...T) T { return values[r.IntN(len(values))] }

func randomAction(r *rand.Rand) *json.RawMessage {
	switch r.IntN(5) {
	case 0:
		return nil
	case 1:
		return raw(pick(r, "fix_request", "retry_later", "use_other_node", "in_progress", "check_status", "IN_PROGRESS", ""))
	case 2:
		return raw(randomString(r))
	case 3:
		return raw(nil)
	}
	return raw(r.Int64())
}

func randomTxHash(r *rand.Rand) *json.RawMessage {
	return pick(r,
		nil, raw(hexOf(ourHash)), raw("0x"+strings.ToUpper(hexOf(ourHash)[2:])), raw(hexOf(ourHash)[2:]),
		raw(hexOf(anotherHash)), raw(nil), raw(r.Uint64()), raw(randomString(r)))
}

func foreignType(r *rand.Rand) *json.RawMessage {
	switch r.IntN(4) {
	case 0:
		return nil
	case 1:
		return pick(r, raw(nil), raw(7))
	case 2:
		return raw(pick(r,
			"https://httpproblems.com/http-status/429", "urn:future", "https://developer.1moneynetwork.com/problem",
			"https://developer.1moneynetwork.com/problems/nonce_too_low",
			"HTTPS://DEVELOPER.1MONEYNETWORK.COM/problem/nonce_too_low", "about:blank#x", ""))
	}
	for {
		s := randomString(r)
		if !strings.HasPrefix(s, ProblemTypePrefix) && s != "about:blank" {
			return raw(s)
		}
	}
}

func randomType(r *rand.Rand) *json.RawMessage {
	switch r.IntN(4) {
	case 0:
		return raw(newerNodeType)
	case 1:
		return raw(inFlightType)
	case 2:
		return raw("about:blank")
	}
	return foreignType(r)
}

func randomStatus(r *rand.Rand) int {
	if r.IntN(2) == 0 {
		return pick(r, 100, 199, 200, 299, 300, 399, 400, 407, 408, 409, 421, 422, 423, 499, 500, 599)
	}
	return 100 + r.IntN(500)
}

func randomMessage(r *rand.Rand) string {
	reasons := []string{
		"Failed to recover REST envelope: invalid signature bytes", "Zero sender",
		"already inflight; please retry later", "nonce already used; state advanced",
	}
	switch r.IntN(3) {
	case 0:
		return randomString(r)
	case 1:
		return pinned(pick(r, reasons...))
	}
	return pick(r, reasons...)
}

func randomBody(r *rand.Rand) propertyBody {
	return propertyBody{
		hash:      pick(r, nil, raw(hexOf(ourHash)), raw(1)),
		typeURI:   randomType(r),
		action:    randomAction(r),
		txHash:    randomTxHash(r),
		errorCode: pick(r, nil, raw("business_transaction_failed"), raw(randomString(r)), raw(422)),
		message:   randomMessage(r),
	}
}

func (b propertyBody) render() string {
	members := map[string]*json.RawMessage{"hash": b.hash, "type": b.typeURI, "action": b.action, "tx_hash": b.txHash, "error_code": b.errorCode}
	object := map[string]any{"message": b.message}
	for name, value := range members {
		if value != nil {
			object[name] = *value
		}
	}
	out, _ := json.Marshal(object)
	return string(out)
}

func asString(v *json.RawMessage) (string, bool) {
	var s string
	if v == nil || json.Unmarshal(*v, &s) != nil {
		return "", false
	}
	return s, true
}

// expectedKind states the safety property as conditions, not as ordered rules.
func expectedKind(status int, b propertyBody) submitOutcomeKind {
	_, hasStringHash := asString(b.hash)
	typeURI, typed := asString(b.typeURI)
	nodeType := typed && (strings.HasPrefix(typeURI, ProblemTypePrefix) || typeURI == "about:blank")
	action, hasAction := asString(b.action)
	hasAction = hasAction && nodeType
	inProgressHere := hasAction && action == "in_progress"
	if inProgressHere && b.txHash != nil {
		txHash, ok := asString(b.txHash)
		inProgressHere = ok && (strings.EqualFold(txHash, hexOf(ourHash)) || strings.EqualFold(txHash, hexOf(ourHash)[2:]))
	}
	switch {
	case status >= 200 && status <= 299 && hasStringHash:
		return outcomeAccepted
	case hasAction && (action == "fix_request" || action == "retry_later" || action == "use_other_node"):
		return outcomeRejected
	case inProgressHere:
		return outcomeLocked
	}
	return outcomeUnknown
}

const propertyCases = 4000

func TestJSONBodiesFollowTheSafetyProperty(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range propertyCases {
		status, body := randomStatus(r), randomBody(r)
		rendered := body.render()
		first, second := classifyKind(status, rendered), classifyKind(status, rendered)
		if first != second || first != expectedKind(status, body) {
			t.Fatalf("%d %s: %v then %v, want %v", status, rendered, first, second, expectedKind(status, body))
		}
	}
}

func TestMissingOrUnusableActionsCannotProveRejection(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	recognized := map[string]bool{"fix_request": true, "retry_later": true, "use_other_node": true, "in_progress": true, "check_status": true}
	for range propertyCases {
		object := map[string]any{}
		if action := randomAction(r); action != nil {
			if s, ok := asString(action); !ok || !recognized[s] {
				object["action"] = *action
			}
		}
		for _, name := range []string{"error_code", "message", "tx_hash"} {
			if r.IntN(2) == 0 {
				object[name] = randomString(r)
			}
		}
		if typeURI := randomType(r); typeURI != nil {
			object["type"] = *typeURI
		}
		status := 400 + r.IntN(200)
		if got := classifyJSON(t, status, object); got != outcomeUnknown {
			t.Fatalf("%d %v: %v", status, object, got)
		}
	}
}

func TestMessageTextNeverChangesTheOutcome(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	for range propertyCases {
		status, body := randomStatus(r), randomBody(r)
		changed := body
		changed.message = randomString(r)
		if a, b := classifyKind(status, body.render()), classifyKind(status, changed.render()); a != b {
			t.Fatalf("%s vs %s: %v vs %v", body.render(), changed.render(), a, b)
		}
	}
}

func TestAnyTextBodyWithoutAnObjectIsUnknown(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	for range propertyCases {
		status, body := randomStatus(r), randomString(r)
		var object map[string]json.RawMessage
		if json.Unmarshal([]byte(body), &object) == nil && object != nil {
			continue
		}
		if got := classifyKind(status, body); got != outcomeUnknown {
			t.Fatalf("%d %q: %v", status, body, got)
		}
	}
}

func TestATransportFailureIsAlwaysUnknown(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	for range 200 {
		status := pick(r, 0, randomStatus(r))
		if got := classifySubmit(answer{status: status, err: errors.New("x")}, ourHash).kind; got != outcomeUnknown {
			t.Fatalf("%d: %v", status, got)
		}
	}
}

func TestAnswersWithoutTheNodesStatementClassifyAsAnEmptyBody(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	for range propertyCases {
		status := randomStatus(r)
		object := map[string]any{"message": randomMessage(r)}
		for name, value := range map[string]*json.RawMessage{"type": foreignType(r), "action": randomAction(r), "tx_hash": randomTxHash(r)} {
			if value != nil {
				object[name] = *value
			}
		}
		withBody := classifySubmit(httpAnswer(status, jsonBody(t, object)), ourHash)
		empty := classifySubmit(httpAnswer(status, ""), ourHash)
		if withBody.kind != empty.kind || withBody.nonceUsed != empty.nonceUsed {
			t.Fatalf("%d %v: %+v vs %+v", status, object, withBody, empty)
		}
		if a, b := readTransaction(httpAnswer(status, jsonBody(t, object)), ourHash).kind, readTransaction(httpAnswer(status, ""), ourHash).kind; a != b {
			t.Fatalf("read %d %v: %v vs %v", status, object, a, b)
		}
	}
}

func FuzzClassifySubmit(f *testing.F) {
	f.Add(422, `{"type":"`+inFlightType+`","action":"in_progress"}`)
	f.Add(200, `{"hash":"`+hexOf(ourHash)+`"}`)
	f.Add(503, `{"type":"about:blank","action":"retry_later"}`)
	f.Fuzz(func(t *testing.T, status int, body string) {
		first := classifySubmit(httpAnswer(status, body), ourHash)
		second := classifySubmit(httpAnswer(status, body), ourHash)
		if first.kind != second.kind || first.nonceUsed != second.nonceUsed || first.refusal != second.refusal {
			t.Fatalf("not deterministic: %+v vs %+v", first, second)
		}
		var fields struct {
			Type   *string          `json:"type"`
			Action *json.RawMessage `json:"action"`
		}
		_ = json.Unmarshal([]byte(body), &fields)
		if first.kind == outcomeRejected || first.kind == outcomeLocked {
			if fields.Type == nil || (!strings.HasPrefix(*fields.Type, ProblemTypePrefix) && *fields.Type != "about:blank") || fields.Action == nil {
				t.Fatalf("%v without the node's action: %s", first.kind, body)
			}
		}
	})
}
