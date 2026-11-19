package onemoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// fakeClock moves only when the run sleeps or a scripted request takes time.
// cancelAt ends the run's context at that time, also during a wait.
type fakeClock struct {
	t        time.Time
	cancelAt *time.Time
	cancel   context.CancelFunc
}

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func (c *fakeClock) sleepUntil(ctx context.Context, at time.Time) bool {
	if ctx.Err() != nil {
		return false
	}
	if c.cancelAt != nil && !c.cancelAt.After(at) {
		if c.cancelAt.After(c.t) {
			c.t = *c.cancelAt
		}
		c.cancel()
		return false
	}
	if at.After(c.t) {
		c.t = at
	}
	return true
}

type callKind string

const (
	callSend    callKind = "send"
	callReceipt callKind = "receipt"
	callLookup  callKind = "lookup"
	callStatus  callKind = "status"
)

type recordedCall struct {
	kind callKind
	at   time.Duration
}

// script is a scripted node. Each request takes the next queued answer; an
// empty queue gives the default: an unknown send, a missing receipt, a
// missing transaction. receiptFrom makes every later receipt read find it.
type script struct {
	clock                         *fakeClock
	started                       time.Time
	sends, receipts, lookups      []answer
	statuses                      []answer
	receiptDefault, lookupDefault func() answer
	receiptFrom                   *time.Duration
	lookupTakes                   time.Duration
	cancelAtReceipt               int
	calls                         []recordedCall
}

func newScript() *script {
	start := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	return &script{clock: &fakeClock{t: start}, started: start, receiptDefault: notFoundAnswer, lookupDefault: notFoundAnswer}
}

func (s *script) withSends(answers ...answer) *script {
	s.sends = append(s.sends, answers...)
	return s
}

func (s *script) withReceipts(answers ...answer) *script {
	s.receipts = append(s.receipts, answers...)
	return s
}

func (s *script) withLookups(answers ...answer) *script {
	s.lookups = append(s.lookups, answers...)
	return s
}

func (s *script) withStatuses(answers ...answer) *script {
	s.statuses = append(s.statuses, answers...)
	return s
}

// lookupsFail: lookups that fail after the queued ones, so there is no probe
// and no early end; the sends follow the retry gate alone.
func (s *script) lookupsFail() *script { s.lookupDefault = failedAnswer; return s }

// receiptsFail: receipt reads that fail after the queued ones, so a nonce
// answer runs to its cap.
func (s *script) receiptsFail() *script { s.receiptDefault = failedAnswer; return s }

func (s *script) receiptFromSecs(at float64) *script {
	d := secsOf(at)
	s.receiptFrom = &d
	return s
}

func (s *script) record(kind callKind) int {
	s.calls = append(s.calls, recordedCall{kind, s.clock.t.Sub(s.started)})
	count := 0
	for _, c := range s.calls {
		if c.kind == kind {
			count++
		}
	}
	return count
}

func (s *script) times(kind callKind) []time.Duration {
	var out []time.Duration
	for _, c := range s.calls {
		if c.kind == kind {
			out = append(out, c.at)
		}
	}
	return out
}

func nextAnswer(queue *[]answer, fallback func() answer) answer {
	if len(*queue) == 0 {
		return fallback()
	}
	a := (*queue)[0]
	*queue = (*queue)[1:]
	return a
}

func (s *script) send(_ context.Context, authorized *AuthorizedTransaction) (submitOutcome, error) {
	s.record(callSend)
	return classifySubmit(nextAnswer(&s.sends, unknownAnswer), common.BytesToHash(authorized.txHash)), nil
}

func (s *script) receipt(_ context.Context, hash common.Hash) (readResult[*TransactionReceiptResponse], error) {
	count := s.record(callReceipt)
	if s.cancelAtReceipt == count {
		s.clock.cancel()
	}
	if s.receiptFrom != nil && s.clock.t.Sub(s.started) >= *s.receiptFrom {
		return readReceipt(receiptAnswer(hash), hash), nil
	}
	return readReceipt(nextAnswer(&s.receipts, s.receiptDefault), hash), nil
}

func (s *script) lookup(_ context.Context, hash common.Hash) (readResult[struct{}], error) {
	s.record(callLookup)
	s.clock.advance(s.lookupTakes)
	return readTransaction(nextAnswer(&s.lookups, s.lookupDefault), hash), nil
}

func (s *script) status(_ context.Context, hash common.Hash) (readResult[TransactionStatus], error) {
	s.record(callStatus)
	return readStatus(nextAnswer(&s.statuses, notFoundAnswer), hash), nil
}

func jsonAnswer(status int, members map[string]any) answer {
	body, err := json.Marshal(members)
	if err != nil {
		panic(err)
	}
	return answer{status: status, body: body}
}

func emptyAnswer(status int) answer { return answer{status: status} }
func acceptedAnswer(hash common.Hash) answer {
	return jsonAnswer(200, map[string]any{"hash": hash.Hex()})
}
func unknownAnswer() answer { return emptyAnswer(500) }
func failedAnswer() answer  { return emptyAnswer(502) }

func inFlightAnswer() answer {
	return jsonAnswer(422, map[string]any{"type": inFlightType, "title": "Submission already in flight", "status": 422, "action": "in_progress"})
}

func nonceTooLowTypedAnswer(typeURI string) answer {
	return jsonAnswer(422, map[string]any{
		"type": typeURI, "title": "Nonce too low", "status": 422, "action": "check_status",
		"error_code": "business_transaction_failed", "message": "nonce too low",
	})
}

func nonceTooLowAnswer() answer { return nonceTooLowTypedAnswer(nonceTooLowType) }

func legacyAnswer(message string) answer {
	return jsonAnswer(422, map[string]any{"error_code": "business_transaction_failed", "message": message})
}

func legacyInsufficientBalance() answer {
	return legacyAnswer(pinned("Insufficient balance: required 10, available 1"))
}

func legacyNonceUsed() answer { return legacyAnswer(pinned("nonce already used; state advanced")) }

func legacyInFlight() answer { return legacyAnswer(pinned("already inflight; please retry later")) }

func legacyBytesRefused() answer {
	return legacyAnswer(pinned("Invalid chain id: expected 1212101, actual 1"))
}

func chainIDMismatchAnswer() answer {
	return jsonAnswer(422, map[string]any{"type": ProblemTypePrefix + "chain_id_mismatch", "title": "Chain ID mismatch", "status": 422, "action": "fix_request"})
}

func complianceBlockedAnswer() answer {
	return jsonAnswer(403, map[string]any{"type": ProblemTypePrefix + "compliance_blocked", "title": "Blocked by compliance", "status": 403, "action": "fix_request"})
}

func retryLaterAnswer() answer {
	return jsonAnswer(503, map[string]any{"type": "about:blank", "title": "busy", "action": "retry_later"})
}

func rateLimitedAnswer(retryAfterSecs uint64) answer {
	return jsonAnswer(429, map[string]any{
		"type": ProblemTypePrefix + "rate_limited", "title": "Rate limit exceeded", "status": 429,
		"action": "retry_later", "retry_after_secs": retryAfterSecs,
	})
}

func firewall429() answer {
	return jsonAnswer(429, map[string]any{"error_code": "rate_limit_exceeded", "message": "Too many requests"})
}

func receiptAnswer(hash common.Hash) answer {
	return answer{status: 200, body: []byte(receiptBody(hash))}
}

func unreadableReceiptAnswer(hash common.Hash) answer {
	return answer{status: 200, body: []byte(unreadableReceiptBody(hash))}
}

func transactionAnswer(hash common.Hash) answer {
	return jsonAnswer(200, map[string]any{"hash": hash.Hex()})
}

func notFoundAnswer() answer {
	return jsonAnswer(404, map[string]any{"error_code": "resource_transaction_not_found", "message": "not found"})
}

func statusAnswer(hash common.Hash, status string) answer {
	return jsonAnswer(200, map[string]any{"hash": hash.Hex(), "status": status})
}

func repeated(n int, f func() answer) []answer {
	out := make([]answer, n)
	for i := range out {
		out[i] = f()
	}
	return out
}

func secsOf(v float64) time.Duration { return time.Duration(v * float64(time.Second)) }

func secs(values ...float64) []time.Duration {
	out := make([]time.Duration, len(values))
	for i, v := range values {
		out[i] = secsOf(v)
	}
	return out
}

func withDeadline(seconds int) SettleOptions {
	o := DefaultSettleOptions()
	o.Deadline = time.Duration(seconds) * time.Second
	return o
}

type runOptions struct {
	authorized *AuthorizedTransaction
	start      settleStart
	options    SettleOptions
	cancelAt   *time.Duration
	preCancel  bool
}

func (s *script) run(t *testing.T, authorized *AuthorizedTransaction, start settleStart, options SettleOptions) (*SettledTransaction, error) {
	t.Helper()
	return s.runWith(runOptions{authorized: authorized, start: start, options: options})
}

func (s *script) runWith(o runOptions) (*SettledTransaction, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.clock.cancel = cancel
	if o.cancelAt != nil {
		at := s.started.Add(*o.cancelAt)
		s.clock.cancelAt = &at
	}
	if o.preCancel {
		cancel()
	}
	return runSettle(ctx, s, s.clock, o.authorized, o.start, o.options)
}

func (s *script) fail(t *testing.T, authorized *AuthorizedTransaction, start settleStart, options SettleOptions) *SubmitError {
	t.Helper()
	return s.failWith(t, runOptions{authorized: authorized, start: start, options: options})
}

func (s *script) failWith(t *testing.T, o runOptions) *SubmitError {
	t.Helper()
	settled, err := s.runWith(o)
	var submitErr *SubmitError
	if !errors.As(err, &submitErr) {
		t.Fatalf("expected *SubmitError, got %v, %v", settled, err)
	}
	return submitErr
}

func sendResults(err *SubmitError) []ObservedResult {
	var out []ObservedResult
	for _, o := range err.Observations {
		if o.Request == ObservedSend {
			out = append(out, o.Result)
		}
	}
	return out
}

func assertTimes(t *testing.T, what string, got, want []time.Duration) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s at %v, want %v", what, got, want)
	}
}

func assertKind(t *testing.T, err *SubmitError, kind SubmitErrorKind) {
	t.Helper()
	if err.Kind != kind {
		t.Fatalf("kind %s, want %s: %v", err.Kind, kind, err)
	}
}

func assertEnd(t *testing.T, err *SubmitError, reason SettleEndReason) {
	t.Helper()
	if err.EndReason != reason {
		t.Fatalf("end %q, want %q: %v", err.EndReason, reason, err)
	}
}

func last(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return -1
	}
	return ds[len(ds)-1]
}

type settleFixture struct {
	payment, settled, lane *AuthorizedTransaction
}

func fixtures(t *testing.T) settleFixture {
	t.Helper()
	settled, _ := authorizedBody(t, settledVectorPayload(SettledPause{}))
	lane, _ := authorizedBody(t, laneVectorPayload())
	return settleFixture{payment: authorizedPayment(t), settled: settled, lane: lane}
}

func hashOf(a *AuthorizedTransaction) common.Hash { return common.BytesToHash(a.txHash) }

// --- Sends, states and refusals ---

func TestAnAcceptedSendSettlesAtItsReceipt(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.payment)
	s := newScript().withSends(acceptedAnswer(h)).withLookups(transactionAnswer(h)).receiptFromSecs(2)
	settled, err := s.run(t, f.payment, startClean, DefaultSettleOptions())
	if err != nil || settled.TransactionHash != h || settled.Receipt.TransactionHash != h.Hex() {
		t.Fatalf("%+v %v", settled, err)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1, 2))
	assertTimes(t, "a found lookup is locked: no more lookups", s.times(callLookup), secs(0.5))
}

func TestAnAcceptedSendIsNotLockedAndEndsUnknownAfterTheProbes(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(acceptedAnswer(hashOf(f.payment)))
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 3, 7))
	if last(s.times(callReceipt)) != 9*time.Second {
		t.Fatalf("last receipt %v", last(s.times(callReceipt)))
	}
}

func TestAnUnknownSendIsProbedAndEndsUnknownAfterThreeMissedLookups(t *testing.T) {
	f := fixtures(t)
	s := newScript()
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	if err.NonceUsed {
		t.Fatal("nonce used")
	}
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 3, 7))
	lookups := s.times(callLookup)
	assertTimes(t, "the lookup 0.5 s after the last probe does not count", lookups[len(lookups)-4:], secs(7.5, 8, 8.5, 9))
}

func TestAFailedLookupRestartsTheMissedCount(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(append(repeated(17, notFoundAnswer), failedAnswer())...)
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	if last(s.times(callReceipt)) != secsOf(10.5) {
		t.Fatalf("last receipt %v", last(s.times(callReceipt)))
	}
}

func TestASendRestartsTheMissedCount(t *testing.T) {
	f := fixtures(t)
	options := DefaultSettleOptions()
	options.ResendInterval = secsOf(8.5)
	s := newScript()
	err := s.fail(t, f.payment, startClean, options)
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 3, 7, 8.5))
	if last(s.times(callReceipt)) != secsOf(10.5) {
		t.Fatalf("last receipt %v", last(s.times(callReceipt)))
	}

	s = newScript().withSends(unknownAnswer(), unknownAnswer(), unknownAnswer(), unknownAnswer(), nonceTooLowAnswer())
	err = s.fail(t, f.payment, startClean, options)
	assertEnd(t, err, SettleEndNonceUsed)
	if last(s.times(callReceipt)) != 10*time.Second {
		t.Fatalf("last receipt %v", last(s.times(callReceipt)))
	}
}

func TestAThrottledReceiptReadRestartsTheMissedCount(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(append(repeated(17, notFoundAnswer), rateLimitedAnswer(1))...)
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	if last(s.times(callReceipt)) != 30*time.Second {
		t.Fatalf("last receipt %v", last(s.times(callReceipt)))
	}
}

func TestTheProbeGapIsMeasuredAtTheLookupStart(t *testing.T) {
	f := fixtures(t)
	s := newScript()
	s.lookupTakes = 600 * time.Millisecond
	_ = s.fail(t, f.payment, startClean, withDeadline(3))
	assertTimes(t, "sends", s.times(callSend)[:2], []time.Duration{0, 2200 * time.Millisecond})
}

func TestAGateSendIsNotAProbe(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(repeated(12, failedAnswer)...)
	_ = s.fail(t, f.payment, startClean, withDeadline(87))
	assertTimes(t, "sends", s.times(callSend), secs(0, 70, 80, 82, 86))
}

func TestALockedTransactionIsResentEveryInterval(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer(), inFlightAnswer(), inFlightAnswer())
	_ = s.fail(t, f.payment, startClean, withDeadline(300))
	assertTimes(t, "sends", s.times(callSend), secs(0, 70, 140, 210, 280))
}

func TestTheResendIntervalRestartsWhenTheTransactionLocks(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.payment)
	s := newScript().withSends(acceptedAnswer(h)).withLookups(notFoundAnswer(), transactionAnswer(h))
	_ = s.fail(t, f.payment, startClean, withDeadline(150))
	assertTimes(t, "sends", s.times(callSend), secs(0, 71, 141))
}

func TestAProbeAnsweredInProgressLocksAndRestartsTheGate(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(acceptedAnswer(hashOf(f.payment)), inFlightAnswer())
	err := s.fail(t, f.payment, startClean, withDeadline(75))
	assertKind(t, err, SubmitLocked)
	assertEnd(t, err, SettleEndDeadline)
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 71))
	assertTimes(t, "lookups", s.times(callLookup), secs(0.5, 1))
}

func TestALegacyInFlightTextStaysUnknown(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), legacyInFlight())
	err := s.fail(t, f.payment, startClean, withDeadline(10))
	assertKind(t, err, SubmitOutcomeUnknown)
	if got := sendResults(err); !reflect.DeepEqual(got, []ObservedResult{ObservedUnknown, ObservedUnknown, ObservedUnknown, ObservedUnknown}) {
		t.Fatalf("%v", got)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 3, 7))
}

func TestAReceiptThatDoesNotDecodeEndsTheRunAtOnce(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(unreadableReceiptAnswer(hashOf(f.payment)))
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndReceiptUnreadable)
	if err.RequestHadNoEffect() || len(s.calls) != 1 || s.calls[0].kind != callReceipt {
		t.Fatalf("%v %v", err, s.calls)
	}
	if len(err.Observations) != 1 || err.Observations[0].Result != ObservedFound {
		t.Fatalf("the receipt exists: %+v", err.Observations)
	}
}

func TestAReceiptThatDoesNotDecodeIsOutcomeUnknownAlsoWhenLocked(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer()).withReceipts(notFoundAnswer(), unreadableReceiptAnswer(hashOf(f.payment)))
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndReceiptUnreadable)
}

func TestAReceiptThatDoesNotDecodeAfterNonceTooLowIsNotNonceUsed(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer()).withReceipts(notFoundAnswer(), unreadableReceiptAnswer(hashOf(f.payment)))
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndReceiptUnreadable)
}

func TestACheckpointSettledOperationWhoseNonceIsUsedIsSettlingUntilItsReceipt(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.settled)
	certified := func() answer { return statusAnswer(h, "certified_pending_checkpoint") }
	s := newScript().withSends(nonceTooLowAnswer()).withStatuses(certified()).receiptFromSecs(120)
	settled, err := s.run(t, f.settled, startClean, DefaultSettleOptions())
	if err != nil || settled.TransactionHash != h {
		t.Fatalf("the receipt appears at the checkpoint: %v %v", settled, err)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0))
	if len(s.times(callStatus)) != 1 {
		t.Fatalf("once certified, only the receipt is read: %v", s.times(callStatus))
	}

	s = newScript().withSends(nonceTooLowAnswer()).withStatuses(certified())
	submitErr := s.fail(t, f.settled, startClean, withDeadline(300))
	assertEnd(t, submitErr, SettleEndSettling)
	if !submitErr.NonceUsed {
		t.Fatal("nonce used")
	}
	assertTimes(t, "sends", s.times(callSend), secs(0))
}

func TestALaneClawbackWhoseSlotIsUsedIsSettlingOnceANodeReportsItCounterSigned(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.lane)
	s := newScript().withSends(nonceTooLowAnswer()).withStatuses(append(repeated(20, notFoundAnswer), statusAnswer(h, "finalized"))...)
	err := s.fail(t, f.lane, startClean, withDeadline(900))
	assertEnd(t, err, SettleEndSettling)
	assertKind(t, err, SubmitOutcomeUnknown)
	if len(s.times(callStatus)) != 21 {
		t.Fatalf("%d status reads", len(s.times(callStatus)))
	}
}

func TestANodeWithoutTheHistoryNeverEndsAnEvidenceOnlyRunAsNonceUsed(t *testing.T) {
	f := fixtures(t)
	for _, authorized := range []*AuthorizedTransaction{f.settled, f.lane} {
		s := newScript().withSends(nonceTooLowAnswer())
		err := s.fail(t, authorized, startClean, withDeadline(900))
		assertEnd(t, err, SettleEndNonceUsedOutcomeUnknown)
		assertKind(t, err, SubmitOutcomeUnknown)
		if !err.NonceUsed {
			t.Fatalf("%s: nonce used", authorized.path)
		}
		assertTimes(t, authorized.path+": no resend", s.times(callSend), secs(0))
		if lastStatus := last(s.times(callStatus)); lastStatus < 900*time.Second-defaultMaxPollInterval {
			t.Fatalf("%s: the reads go on until the deadline: %v", authorized.path, lastStatus)
		}
	}
}

func TestAPooledOrUnansweredStatusNeverProvesAnythingAfterNonceTooLow(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.settled)
	for _, status := range []func() answer{func() answer { return statusAnswer(h, "pending") }, failedAnswer} {
		s := newScript().withSends(nonceTooLowAnswer()).withStatuses(repeated(1000, status)...)
		err := s.fail(t, f.settled, startClean, withDeadline(300))
		assertEnd(t, err, SettleEndNonceUsedOutcomeUnknown)
		if !err.NonceUsed {
			t.Fatal("nonce used")
		}
	}
}

func TestALockedEvidenceOnlyRunWhoseResendFindsTheNonceUsedIsNotLeftLocked(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer(), nonceTooLowAnswer())
	err := s.fail(t, f.lane, startClean, withDeadline(300))
	assertEnd(t, err, SettleEndNonceUsedOutcomeUnknown)
	assertKind(t, err, SubmitOutcomeUnknown)
}

func TestALiveTransactionNeverReadsTheStatus(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer())
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertEnd(t, err, SettleEndNonceUsed)
	if len(s.times(callStatus)) != 0 {
		t.Fatal("status read")
	}
}

func TestARejectedFirstSendIsRejected(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(complianceBlockedAnswer())
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitRejected)
	if !err.RequestHadNoEffect() || err.Refusal != SubmitRefusalRequest || err.Answer.Status != 403 || len(s.calls) != 1 {
		t.Fatalf("%+v %v", err, s.calls)
	}
}

func TestAFirstSendEndsRejectedOnlyWhenTheClassifierRejects(t *testing.T) {
	f := fixtures(t)
	for _, first := range []func() answer{complianceBlockedAnswer, retryLaterAnswer} {
		s := newScript().withSends(first())
		err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
		assertKind(t, err, SubmitRejected)
		if !err.RequestHadNoEffect() || len(s.calls) != 1 {
			t.Fatalf("%s: %v", first().body, s.calls)
		}
	}
	for _, tc := range []struct {
		first     answer
		nonceUsed bool
	}{
		{legacyInsufficientBalance(), false},
		{legacyAnswer("nonce too low"), false},
		{legacyNonceUsed(), false},
		{legacyBytesRefused(), false},
		{nonceTooLowAnswer(), true},
		{nonceTooLowTypedAnswer(nonceTooLowType + "x"), false},
		{emptyAnswer(409), false},
	} {
		s := newScript().withSends(tc.first)
		err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
		assertKind(t, err, SubmitOutcomeUnknown)
		if err.RequestHadNoEffect() || err.NonceUsed != tc.nonceUsed {
			t.Fatalf("%s: %+v", tc.first.body, err)
		}
		if sends := s.times(callSend); len(sends) > 1 && sends[1] < time.Second {
			t.Fatalf("no resend follows the first answer at once: %v", sends)
		}
	}
}

func TestAFirstSend408OrInProgressForAnotherHashIsNotRejected(t *testing.T) {
	f := fixtures(t)
	for _, first := range []answer{
		emptyAnswer(408),
		jsonAnswer(422, map[string]any{"type": inFlightType, "action": "in_progress", "tx_hash": anotherHash.Hex()}),
	} {
		s := newScript().withSends(first).lookupsFail()
		err := s.fail(t, f.payment, startClean, withDeadline(10))
		assertKind(t, err, SubmitOutcomeUnknown)
	}
}

func TestAFirstSendLegacyRefusalSettlesAndKeepsTheNodeMessage(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(legacyInsufficientBalance())
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 3, 7))
	first := err.Observations[0]
	if first.Status != 422 || first.Message != pinned("Insufficient balance: required 10, available 1") {
		t.Fatalf("%+v", first)
	}
}

func TestARequestRefusalOfAResendEndsNothing(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), complianceBlockedAnswer()).lookupsFail()
	err := s.fail(t, f.payment, startClean, withDeadline(100))
	assertKind(t, err, SubmitOutcomeUnknown)
	if err.RequestHadNoEffect() || !reflect.DeepEqual(sendResults(err), []ObservedResult{ObservedUnknown, ObservedRejected}) {
		t.Fatalf("%v", sendResults(err))
	}
	for _, o := range err.Observations {
		if o.Result != ObservedRejected {
			continue
		}
		if o.Status != 403 || string(o.Action) != `"fix_request"` || o.ProblemType != ProblemTypePrefix+"compliance_blocked" ||
			o.Message != "Blocked by compliance" || o.Elapsed != 70*time.Second {
			t.Fatalf("%+v", o)
		}
	}
	if err.Answer.Status != 403 {
		t.Fatalf("answer %+v", err.Answer)
	}
}

func TestARequestRefusalOnSettleEndsNothing(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(complianceBlockedAnswer()).lookupsFail()
	err := s.fail(t, f.payment, startSent, withDeadline(10))
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndDeadline)
}

func TestABytesRefusalWhileUnknownIsRejectedOnAnySend(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(chainIDMismatchAnswer())
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertKind(t, err, SubmitRejected)
	if err.Refusal != SubmitRefusalBytes {
		t.Fatalf("%v", err.Refusal)
	}
	s = newScript().withSends(unknownAnswer(), chainIDMismatchAnswer())
	err = s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitRejected)
	if err.Refusal != SubmitRefusalBytes {
		t.Fatalf("%v", err.Refusal)
	}
	assertTimes(t, "the probe got it", s.times(callSend), secs(0, 1))
}

func TestABytesRefusalAfterAnAcceptedSendOrAFoundLookupEndsNothing(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.payment)
	s := newScript().withSends(acceptedAnswer(h), chainIDMismatchAnswer()).lookupsFail()
	err := s.fail(t, f.payment, startClean, withDeadline(100))
	assertKind(t, err, SubmitOutcomeUnknown)
	if !reflect.DeepEqual(sendResults(err), []ObservedResult{ObservedAccepted, ObservedRejected}) {
		t.Fatalf("%v", sendResults(err))
	}
	s = newScript().withSends(unknownAnswer(), chainIDMismatchAnswer()).withLookups(transactionAnswer(h))
	err = s.fail(t, f.payment, startClean, withDeadline(100))
	assertKind(t, err, SubmitLocked)
	assertTimes(t, "sends", s.times(callSend), secs(0, 70.5))
}

func TestAnotherHashOnAResendIsAHashMismatch(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), acceptedAnswer(anotherHash))
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitHashMismatch)
	if err.TransactionHash != hashOf(f.payment) || err.ServerHash != anotherHash.Hex() {
		t.Fatalf("%+v", err)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0, 1))
}

func TestAResumeThatIsInProgressSettlesAtItsReceipt(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer()).receiptFromSecs(0.5)
	settled, err := s.run(t, f.payment, startSent, DefaultSettleOptions())
	if err != nil || settled.TransactionHash != hashOf(f.payment) {
		t.Fatalf("%v %v", settled, err)
	}
	assertTimes(t, "receipts", s.times(callReceipt), secs(0, 0.5))
	assertTimes(t, "lookups", s.times(callLookup), secs(0))
	assertTimes(t, "sends", s.times(callSend), secs(0))
}

func TestAFoundLookupIsLocked(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(transactionAnswer(hashOf(f.payment)))
	err := s.fail(t, f.payment, startClean, withDeadline(30))
	assertKind(t, err, SubmitLocked)
	assertTimes(t, "lookups", s.times(callLookup), secs(0.5))
}

func TestLockedNeverReturnsToUnknown(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer(), unknownAnswer())
	err := s.fail(t, f.payment, startClean, withDeadline(100))
	assertKind(t, err, SubmitLocked)
	if !reflect.DeepEqual(sendResults(err), []ObservedResult{ObservedLocked, ObservedUnknown}) {
		t.Fatalf("%v", sendResults(err))
	}
}

func TestAnotherHashIsNeverFound(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(receiptAnswer(anotherHash), receiptAnswer(anotherHash)).
		withLookups(transactionAnswer(anotherHash), transactionAnswer(anotherHash))
	err := s.fail(t, f.payment, startClean, withDeadline(10))
	assertKind(t, err, SubmitOutcomeUnknown)
	for _, o := range err.Observations {
		if o.Result == ObservedFound {
			t.Fatalf("found: %+v", o)
		}
	}
}

func TestASendObservationCarriesTheNodeMessage(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer())
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	for _, o := range err.Observations {
		switch {
		case o.Request == ObservedSend && o.Message != "nonce too low":
			t.Fatalf("send: %+v", o)
		case o.Request != ObservedSend && o.Message != "":
			t.Fatalf("read: %+v", o)
		}
	}
}

// --- nonce_too_low ---

func TestNonceTooLowStopsSendsAndAReceiptAfterItSettles(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), nonceTooLowAnswer()).lookupsFail().receiptFromSecs(70.5)
	if _, err := s.run(t, f.payment, startClean, DefaultSettleOptions()); err != nil {
		t.Fatal(err)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0, 70))
	if last(s.times(callReceipt)) != secsOf(70.5) {
		t.Fatalf("%v", last(s.times(callReceipt)))
	}
}

func TestNonceTooLowEndsAfterThreeMissedReceiptReadsWithoutLookups(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), nonceTooLowAnswer()).lookupsFail()
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNonceUsed)
	if !err.NonceUsed {
		t.Fatal("nonce used")
	}
	assertTimes(t, "sends", s.times(callSend), secs(0, 70))
	receipts := s.times(callReceipt)
	assertTimes(t, "receipts", receipts[len(receipts)-3:], secs(70.5, 71, 71.5))
	if last(s.times(callLookup)) != 70*time.Second {
		t.Fatalf("%v", last(s.times(callLookup)))
	}
	var lastSend SettleObservation
	for _, o := range err.Observations {
		if o.Request == ObservedSend {
			lastSend = o
		}
	}
	if lastSend.ProblemType != nonceTooLowType {
		t.Fatalf("%+v", lastSend)
	}
}

func TestAFailedReceiptReadRestartsTheNonceCount(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer()).withReceipts(notFoundAnswer(), notFoundAnswer(), notFoundAnswer(), failedAnswer())
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertEnd(t, err, SettleEndNonceUsed)
	assertTimes(t, "receipts", s.times(callReceipt), secs(0, 0.5, 1, 1.5, 2, 2.5, 3))
}

func TestNonceTooLowWithFailingReceiptReadsEndsAtTheCap(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), nonceTooLowAnswer()).lookupsFail().receiptsFail()
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertEnd(t, err, SettleEndNonceUsed)
	assertTimes(t, "sends", s.times(callSend), secs(0, 70))
	if last(s.times(callReceipt)) != 140*time.Second {
		t.Fatalf("%v", last(s.times(callReceipt)))
	}
}

func TestTheNonceCapIsCutAtTheDeadline(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), nonceTooLowAnswer()).lookupsFail().receiptsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(100))
	if last(s.times(callReceipt)) != 100*time.Second {
		t.Fatalf("%v", last(s.times(callReceipt)))
	}
}

func TestNonceTooLowStopsSendsForACapLongerThanTheResendInterval(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), nonceTooLowAnswer()).lookupsFail().receiptsFail()
	options := DefaultSettleOptions()
	options.NonceUsedGrace = 300 * time.Second
	_ = s.fail(t, f.payment, startClean, options)
	assertTimes(t, "sends", s.times(callSend), secs(0, 70))
	if last(s.times(callReceipt)) != 370*time.Second {
		t.Fatalf("%v", last(s.times(callReceipt)))
	}
}

func TestAFirstSendNonceTooLowSettlesByTheReceiptOnly(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer())
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNonceUsed)
	if !err.NonceUsed || err.RequestHadNoEffect() || err.Problem() == nil || err.Problem().Type != nonceTooLowType {
		t.Fatalf("%+v", err)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1, 1.5))
	if len(s.times(callLookup)) != 0 {
		t.Fatal("lookups")
	}
}

func TestANonceUsedEndIsNeverLocked(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer(), nonceTooLowAnswer()).lookupsFail()
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNonceUsed)
	if !err.NonceUsed {
		t.Fatal("nonce used")
	}
}

// --- Throttled sends ---

func TestAThrottledProbeIsNotCountedAndHoldsTheSends(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), retryLaterAnswer())
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 61, 63, 67))
	inHold := 0
	for _, at := range s.times(callReceipt) {
		if at > time.Second && at < 61*time.Second {
			inHold++
		}
	}
	if inHold < 1 || inHold >= 20 {
		t.Fatalf("%d receipt reads while sends are held", inHold)
	}
}

func TestAThrottledSendHoldsForTheNodeHint(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(unknownAnswer(), rateLimitedAnswer(5))
	_ = s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertTimes(t, "sends", s.times(callSend)[:3], secs(0, 1, 6))
}

func TestAThrottledFirstSendOfSettleDeliversNothing(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(retryLaterAnswer())
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 60, 61, 63, 67))
}

func TestABodylessFourTwoNineHoldsSendsWithoutProvingNonDelivery(t *testing.T) {
	f := fixtures(t)
	for _, sends := range [][]answer{{unknownAnswer(), emptyAnswer(429)}, {emptyAnswer(429)}} {
		s := newScript().withSends(sends...)
		err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
		assertKind(t, err, SubmitOutcomeUnknown)
		if err.RequestHadNoEffect() {
			t.Fatal("no effect")
		}
		times := s.times(callSend)
		throttledAt := times[0]
		if times[1] == time.Second {
			throttledAt = times[1]
		}
		for _, at := range times {
			if at > throttledAt && at < throttledAt+60*time.Second {
				t.Fatalf("send at %v inside the hold: %v", at, times)
			}
		}
	}
}

func TestARateLimitedFirstSendBacksOffAndGoesOn(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(rateLimitedAnswer(5))
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 5, 6, 8, 12))
}

func TestARefusalOfTheFirstSendToReachANodeIsRejected(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(rateLimitedAnswer(5), complianceBlockedAnswer())
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitRejected)
	assertTimes(t, "sends", s.times(callSend), secs(0, 5))
}

func TestNoReadGoesOutBeforeASendReachesANode(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(rateLimitedAnswer(60), rateLimitedAnswer(30))
	_ = s.fail(t, f.payment, startClean, DefaultSettleOptions())
	for _, kind := range []callKind{callReceipt, callLookup} {
		for _, at := range s.times(kind) {
			if at < 90*time.Second {
				t.Fatalf("%s read at %v, before the first delivered send", kind, at)
			}
		}
	}
}

func TestACleanRunThatEndsBeforeASendReachesANodeIsRejected(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(rateLimitedAnswer(60))
	err := s.fail(t, f.payment, startClean, withDeadline(30))
	assertKind(t, err, SubmitRejected)
	if err.Refusal != SubmitRefusalRequest || err.Answer.Status != 429 {
		t.Fatalf("%+v", err)
	}
	assertTimes(t, "sends", s.times(callSend), secs(0))

	s = newScript().withSends(rateLimitedAnswer(60))
	cancelAt := 10 * time.Second
	err = s.failWith(t, runOptions{authorized: f.payment, start: startClean, options: DefaultSettleOptions(), cancelAt: &cancelAt})
	assertKind(t, err, SubmitRejected)
	assertTimes(t, "sends", s.times(callSend), secs(0))
}

func TestAnInProgressAnswerWithStatus429StillLocks(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(jsonAnswer(429, map[string]any{"type": inFlightType, "action": "in_progress", "tx_hash": hashOf(f.payment).Hex()})).lookupsFail()
	err := s.fail(t, f.payment, startClean, withDeadline(75))
	assertKind(t, err, SubmitLocked)
	assertTimes(t, "sends", s.times(callSend), secs(0, 70))
}

func TestEverySendThrottledNeverEndsTheRunEarly(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(repeated(100, func() answer { return rateLimitedAnswer(60) })...)
	err := s.fail(t, f.payment, startSent, withDeadline(300))
	assertEnd(t, err, SettleEndDeadline)
	assertTimes(t, "sends", s.times(callSend), secs(0, 60, 120, 180, 240))
}

func TestEverySendABodylessFourTwoNineEndsUnknownAfterTheProbes(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(repeated(100, func() answer { return emptyAnswer(429) })...)
	err := s.fail(t, f.payment, startSent, withDeadline(300))
	assertKind(t, err, SubmitOutcomeUnknown)
	if err.RequestHadNoEffect() {
		t.Fatal("no effect")
	}
	assertEnd(t, err, SettleEndNotFoundAfterProbes)
	assertTimes(t, "sends", s.times(callSend), secs(0, 60, 120, 180))
}

func TestACleanStartWhereEverySendGetsABodylessFourTwoNineIsNeverRejected(t *testing.T) {
	f := fixtures(t)
	for _, tc := range []struct {
		deadline int
		end      SettleEndReason
	}{{300, SettleEndNotFoundAfterProbes}, {30, SettleEndDeadline}} {
		s := newScript().withSends(repeated(100, func() answer { return emptyAnswer(429) })...)
		err := s.fail(t, f.payment, startClean, withDeadline(tc.deadline))
		assertKind(t, err, SubmitOutcomeUnknown)
		if err.RequestHadNoEffect() {
			t.Fatal("no effect")
		}
		assertEnd(t, err, tc.end)
	}
}

func TestASettleNonceAnswerIsNeverRejected(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer())
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndNonceUsed)
	assertTimes(t, "sends", s.times(callSend), secs(0))
	if last(s.times(callReceipt)) != secsOf(1.5) {
		t.Fatalf("%v", s.times(callReceipt))
	}
}

func TestASettleNonceAnswerWithoutAKnownTypeKeepsTheSchedule(t *testing.T) {
	f := fixtures(t)
	for _, a := range []answer{legacyNonceUsed(), legacyAnswer("nonce too low"), nonceTooLowTypedAnswer(nonceTooLowType + "x")} {
		s := newScript().withSends(a).lookupsFail()
		_ = s.fail(t, f.payment, startSent, withDeadline(211))
		assertTimes(t, string(a.body), s.times(callSend), secs(0, 70, 140, 210))
	}
}

// --- Schedule ---

func TestResendsFollowAFlatInterval(t *testing.T) {
	f := fixtures(t)
	s := newScript().lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(701))
	assertTimes(t, "sends", s.times(callSend), secs(0, 70, 140, 210, 280, 350, 420, 490, 560, 630, 700))
}

func TestProbesDoNotMoveTheGate(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(repeated(14, notFoundAnswer)...).lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(71))
	assertTimes(t, "sends", s.times(callSend), secs(0, 1, 3, 7, 70))
}

func TestThrottledReadsDoNotMoveTheSends(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(repeated(200, func() answer { return emptyAnswer(429) })...)
	_ = s.fail(t, f.payment, startClean, withDeadline(701))
	assertTimes(t, "sends", s.times(callSend), secs(0, 70, 140, 210, 280, 350, 420, 490, 560, 630, 700))
	if len(s.times(callLookup)) != 0 {
		t.Fatal("a throttled receipt read skips the lookup")
	}
}

func TestReadsDoubleFromHalfASecondUpToTenSeconds(t *testing.T) {
	f := fixtures(t)
	s := newScript().lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(30))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1, 2, 4, 8, 16, 26, 30))
}

func TestAfterAThrottledReadAMissedLookupNoLongerShortensTheWait(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(rateLimitedAnswer(1))
	_ = s.fail(t, f.payment, startClean, withDeadline(25))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1.5, 11.5, 21.5, 25))
}

func TestSettleReadsAtOnce(t *testing.T) {
	f := fixtures(t)
	s := newScript().receiptFromSecs(0)
	if _, err := s.run(t, f.payment, startSent, DefaultSettleOptions()); err != nil {
		t.Fatal(err)
	}
	assertTimes(t, "receipts", s.times(callReceipt), secs(0))
	if len(s.times(callSend)) != 0 {
		t.Fatal("sent")
	}
}

func TestTheLastReadIsAtTheDeadline(t *testing.T) {
	f := fixtures(t)
	s := newScript().lookupsFail().receiptFromSecs(3)
	if _, err := s.run(t, f.payment, startClean, withDeadline(3)); err != nil {
		t.Fatal(err)
	}
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1, 2, 3))
}

func TestARunWithoutAReceiptEndsAtTheDeadline(t *testing.T) {
	f := fixtures(t)
	s := newScript().lookupsFail()
	assertEnd(t, s.fail(t, f.payment, startClean, withDeadline(3)), SettleEndDeadline)
}

func TestAThrottledReceiptReadBacksOff(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(emptyAnswer(429)).lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(80))
	assertTimes(t, "receipts", s.times(callReceipt)[:4], secs(0.5, 60.5, 70, 80))
	assertTimes(t, "lookups", s.times(callLookup)[:2], secs(60.5, 70))
}

func TestARetryLaterLookupBacksOff(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(retryLaterAnswer()).lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(65))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 60.5, 65))
}

func TestFailedReadsKeepTheSchedule(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(repeated(10, func() answer { return emptyAnswer(500) })...).
		withLookups(repeated(10, func() answer { return emptyAnswer(502) })...)
	err := s.fail(t, f.payment, startClean, withDeadline(4))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1, 2, 4))
	for _, o := range err.Observations {
		if o.Result == ObservedFailed && o.Status == 500 {
			return
		}
	}
	t.Fatalf("no failed 500 read in %+v", err.Observations)
}

func TestACancelBeforeTheFirstSendSendsNothing(t *testing.T) {
	f := fixtures(t)
	s := newScript()
	_, err := s.runWith(runOptions{authorized: f.payment, start: startClean, options: DefaultSettleOptions(), preCancel: true})
	if err == nil || !strings.Contains(err.Error(), "was not sent") || !errors.Is(err, context.Canceled) || len(s.calls) != 0 {
		t.Fatalf("%v %v", err, s.calls)
	}
}

func TestACancelDuringAReadEndsTheRunAfterIt(t *testing.T) {
	f := fixtures(t)
	s := newScript()
	s.cancelAtReceipt = 2
	err := s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	assertEnd(t, err, SettleEndCancelled)
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 1))
	if len(err.Observations) != 5 {
		t.Fatalf("send, then receipt and lookup twice; the probe due at 1 s is not sent: %+v", err.Observations)
	}
}

func TestACancelAfterNonceTooLowKeepsTheNonceUsed(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(nonceTooLowAnswer())
	s.cancelAtReceipt = 2
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertEnd(t, err, SettleEndCancelled)
	if !err.NonceUsed {
		t.Fatal("nonce used")
	}
}

func TestACancelDuringAWaitEndsTheRunAtOnce(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(inFlightAnswer())
	cancelAt := 5 * time.Second
	err := s.failWith(t, runOptions{authorized: f.payment, start: startClean, options: DefaultSettleOptions(), cancelAt: &cancelAt})
	assertKind(t, err, SubmitLocked)
	assertEnd(t, err, SettleEndCancelled)
	if elapsed := s.clock.t.Sub(s.started); elapsed != 5*time.Second {
		t.Fatalf("ended at %v", elapsed)
	}
}

func TestEveryOptionIsCheckedBeforeAnyRequest(t *testing.T) {
	f := fixtures(t)
	for _, tc := range []struct {
		name string
		set  func(*SettleOptions)
	}{
		{"ResendInterval", func(o *SettleOptions) { o.ResendInterval = 0 }},
		{"FirstReadDelay", func(o *SettleOptions) { o.FirstReadDelay = 0 }},
		{"PollInterval", func(o *SettleOptions) { o.PollInterval = 0 }},
		{"MaxPollInterval", func(o *SettleOptions) { o.MaxPollInterval = 0 }},
		{"Deadline", func(o *SettleOptions) { o.Deadline = 0 }},
		{"ThrottleBackoff", func(o *SettleOptions) { o.ThrottleBackoff = -time.Second }},
		{"NonceUsedGrace", func(o *SettleOptions) { o.NonceUsedGrace = 0 }},
		{"MaxPollInterval", func(o *SettleOptions) { o.MaxPollInterval = o.PollInterval / 2 }},
	} {
		options := DefaultSettleOptions()
		tc.set(&options)
		s := newScript()
		_, err := s.run(t, f.payment, startClean, options)
		if err == nil || !strings.Contains(err.Error(), tc.name) || len(s.calls) != 0 {
			t.Fatalf("%s: %v %v", tc.name, err, s.calls)
		}
	}
	if _, err := runSettle(context.Background(), newScript(), newScript().clock, nil, startClean, DefaultSettleOptions()); err == nil {
		t.Fatal("nil transaction")
	}
}

func TestTheLogKeepsTheNewestTwoHundredObservations(t *testing.T) {
	f := fixtures(t)
	s := newScript()
	options := DefaultSettleOptions()
	options.FirstReadDelay, options.PollInterval, options.MaxPollInterval = time.Millisecond, time.Millisecond, time.Millisecond
	options.Deadline = 500 * time.Millisecond
	err := s.fail(t, f.payment, startClean, options)
	total := len(s.calls)
	if total <= maxObservations || len(err.Observations) != maxObservations {
		t.Fatalf("%d calls, %d observations", total, len(err.Observations))
	}
	if int(err.Observations[0].Seq) != total-maxObservations+1 || int(err.Observations[maxObservations-1].Seq) != total {
		t.Fatalf("seq %d to %d of %d", err.Observations[0].Seq, err.Observations[maxObservations-1].Seq, total)
	}
}

func TestALookupThatEndsAtOrAfterTheDeadlineStartsNoSend(t *testing.T) {
	f := fixtures(t)
	for _, takes := range []float64{1.5, 2} {
		s := newScript()
		s.lookupTakes = secsOf(takes)
		options := DefaultSettleOptions()
		options.ResendInterval, options.Deadline = time.Second, 2*time.Second
		_ = s.fail(t, f.payment, startClean, options)
		assertTimes(t, fmt.Sprintf("lookup takes %v s", takes), s.times(callSend), secs(0))
	}
}

func TestACancelWhileASendIsDueSendsNothing(t *testing.T) {
	f := fixtures(t)
	s := newScript()
	s.cancelAtReceipt = 1
	err := s.fail(t, f.payment, startSent, DefaultSettleOptions())
	assertKind(t, err, SubmitOutcomeUnknown)
	if len(s.times(callSend)) != 0 {
		t.Fatal("the first send of a resume was due at 0 s")
	}
}

func TestAThrottledReadWaitsTheNodeHint(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(rateLimitedAnswer(3)).lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(14))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 3.5, 13.5, 14))
	assertTimes(t, "lookups", s.times(callLookup), secs(3.5, 13.5))
}

func TestAThrottledLookupWaitsTheNodeHint(t *testing.T) {
	f := fixtures(t)
	s := newScript().withLookups(rateLimitedAnswer(5)).lookupsFail()
	_ = s.fail(t, f.payment, startClean, withDeadline(6))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 5.5, 6))
}

func TestAHintBelowThePollIntervalWaitsThePollInterval(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(rateLimitedAnswer(1))
	options := withDeadline(5)
	options.PollInterval = 2 * time.Second
	_ = s.fail(t, f.payment, startClean, options)
	assertTimes(t, "receipts", s.times(callReceipt)[:2], secs(0.5, 2.5))
}

func TestAZeroHintWaitsTheBackOff(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(rateLimitedAnswer(0))
	_ = s.fail(t, f.payment, startClean, withDeadline(100))
	assertTimes(t, "receipts", s.times(callReceipt)[:2], secs(0.5, 60.5))
	s = newScript().withSends(unknownAnswer(), rateLimitedAnswer(0))
	_ = s.fail(t, f.payment, startClean, DefaultSettleOptions())
	assertTimes(t, "sends", s.times(callSend)[:3], secs(0, 1, 61))
}

func TestAHintIsCutAtTheDeadline(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(rateLimitedAnswer(600))
	_ = s.fail(t, f.payment, startClean, withDeadline(30))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 30))
}

func TestAHugeHintWaitsUntilTheDeadline(t *testing.T) {
	f := fixtures(t)
	s := newScript().withReceipts(rateLimitedAnswer(math.MaxUint64))
	_ = s.fail(t, f.payment, startClean, withDeadline(30))
	assertTimes(t, "receipts", s.times(callReceipt), secs(0.5, 30))
}

func TestThePollScheduleDoublesAndRestartsFromTheMaximumAfterAThrottle(t *testing.T) {
	normal, missed := pace{kind: paceNormal}, pace{kind: paceMissed}
	backoff, hint := pace{kind: paceThrottled}, pace{kind: paceThrottled, retryAfter: 2 * time.Second}
	schedule := newPollSchedule(DefaultSettleOptions())
	var waits []time.Duration
	for _, p := range []pace{normal, missed, normal, normal, missed, normal, normal, normal, normal, backoff, normal, missed, hint, normal} {
		waits = append(waits, schedule.afterRead(p))
	}
	assertTimes(t, "waits", waits, secs(0.5, 0.5, 1, 2, 0.5, 4, 8, 10, 10, 60, 10, 10, 2, 10))
	schedule.restart()
	if got := schedule.afterRead(normal); got != 500*time.Millisecond {
		t.Fatalf("%v", got)
	}
}

func TestOnlyAThrottledReadHasAThrottledPace(t *testing.T) {
	if got := paceOf(readReceipt(rateLimitedAnswer(7), ourHash)); got != (pace{kind: paceThrottled, retryAfter: 7 * time.Second}) {
		t.Fatalf("rate limited: %+v", got)
	}
	if got := paceOf(readReceipt(retryLaterAnswer(), ourHash)); got != (pace{kind: paceThrottled}) {
		t.Fatalf("retry later: %+v", got)
	}
	for _, a := range []answer{notFoundAnswer(), emptyAnswer(500), receiptAnswer(ourHash)} {
		if got := paceOf(readReceipt(a, ourHash)); got != (pace{kind: paceNormal}) {
			t.Fatalf("%d: %+v", a.status, got)
		}
	}
}

func TestFirewallBackpressureCannotProveACleanRunHadNoEffect(t *testing.T) {
	f := fixtures(t)
	for _, body := range []string{`{}`, `{"action":null}`, `{"action":"future"}`} {
		s := newScript().withSends(answer{status: 429, body: []byte(body)}).lookupsFail()
		err := s.fail(t, f.payment, startClean, withDeadline(30))
		assertKind(t, err, SubmitOutcomeUnknown)
		if err.RequestHadNoEffect() {
			t.Fatal("no effect")
		}
		assertEnd(t, err, SettleEndDeadline)
		assertTimes(t, "sends", s.times(callSend), secs(0))
		if len(s.times(callReceipt)) == 0 {
			t.Fatal("no receipt read")
		}
	}
}

func TestARefusalAfterAnUnknownFirewallResponseCannotUndoTheFirstSend(t *testing.T) {
	f := fixtures(t)
	s := newScript().withSends(firewall429(), complianceBlockedAnswer()).lookupsFail()
	err := s.fail(t, f.payment, startClean, withDeadline(71))
	assertKind(t, err, SubmitOutcomeUnknown)
	if err.RequestHadNoEffect() {
		t.Fatal("no effect")
	}
	assertTimes(t, "sends", s.times(callSend), secs(0, 70))
}

// --- Properties ---

func TestMissingActionsNeverBecomeRejectionsDuringSettle(t *testing.T) {
	f := fixtures(t)
	r := rand.New(rand.NewPCG(21, 22))
	for range 64 {
		status := 400 + r.IntN(200)
		body := jsonAnswer(status, map[string]any{"error_code": randomString(r), "message": randomString(r)})
		start := pick(r, startClean, startSent)
		s := newScript().withSends(repeated(10, func() answer { return body })...).lookupsFail()
		err := s.fail(t, f.payment, start, withDeadline(3))
		if err.RequestHadNoEffect() || err.Kind != SubmitOutcomeUnknown {
			t.Fatalf("%d %s: %v", status, body.body, err)
		}
	}
}

type sendAnswer int

const (
	sendAccepted sendAnswer = iota
	sendUnknownAnswer
	sendLocked
	sendNonceTooLow
	sendRejected
	sendThrottled
	sendRateLimited
	sendBytesRefused
	sendLegacyRefused
)

func (a sendAnswer) answer(hash common.Hash) answer {
	switch a {
	case sendAccepted:
		return acceptedAnswer(hash)
	case sendLocked:
		return inFlightAnswer()
	case sendNonceTooLow:
		return nonceTooLowAnswer()
	case sendRejected:
		return complianceBlockedAnswer()
	case sendThrottled:
		return retryLaterAnswer()
	case sendRateLimited:
		return rateLimitedAnswer(5)
	case sendBytesRefused:
		return chainIDMismatchAnswer()
	case sendLegacyRefused:
		return legacyInsufficientBalance()
	}
	return unknownAnswer()
}

// model is the send that ends the run as rejected, and the state after the
// sends made: the refusal rules stated as conditions over the answers in order.
func model(answers []sendAnswer, clean bool, sends int) (int, settleState) {
	state, delivered := stateUnknown, 0
	for index := range sends {
		a := sendUnknownAnswer
		if index < len(answers) {
			a = answers[index]
		}
		first := clean && delivered == 0
		rejects := false
		switch a {
		case sendRejected, sendThrottled:
			rejects = first
		case sendBytesRefused:
			rejects = first || state == stateUnknown
		}
		if rejects {
			return index, state
		}
		if a != sendRateLimited && a != sendThrottled {
			delivered++
		}
		switch {
		case a == sendAccepted && state == stateUnknown:
			state = stateAccepted
		case a == sendLocked:
			state = stateLocked
		}
	}
	return -1, state
}

func TestRejectionsAndLocksFollowTheSendAnswers(t *testing.T) {
	f := fixtures(t)
	h := hashOf(f.payment)
	r := rand.New(rand.NewPCG(23, 24))
	for range 64 {
		answers := make([]sendAnswer, 1+r.IntN(7))
		for i := range answers {
			answers[i] = sendAnswer(r.IntN(int(sendLegacyRefused) + 1))
		}
		clean := r.IntN(2) == 0
		s := newScript().lookupsFail()
		for _, a := range answers {
			s.withSends(a.answer(h))
		}
		if r.IntN(2) == 0 {
			s.receiptFromSecs(float64(1 + r.IntN(799)))
		}
		options := withDeadline(900)
		options.NonceUsedGrace = time.Duration(1+r.IntN(399)) * time.Second
		start := startSent
		if clean {
			start = startClean
		}
		_, err := s.run(t, f.payment, start, options)
		sends := len(s.times(callSend))
		for index, a := range answers {
			if a == sendNonceTooLow && sends > index+1 {
				t.Fatalf("%v: %d sends after nonce_too_low at send %d", answers, sends, index+1)
			}
			if a == sendNonceTooLow {
				break
			}
		}
		rejectedAt, state := model(answers, clean, sends)
		var submitErr *SubmitError
		errors.As(err, &submitErr)
		rejected := submitErr != nil && submitErr.Kind == SubmitRejected
		if rejected != (rejectedAt >= 0) || (rejectedAt >= 0 && rejectedAt+1 != sends) {
			t.Fatalf("%v clean=%t: %d sends, model rejects at %d, got %v", answers, clean, sends, rejectedAt, err)
		}
		if submitErr == nil {
			continue
		}
		switch submitErr.Kind {
		case SubmitLocked:
			if state != stateLocked || submitErr.EndReason == SettleEndNonceUsed {
				t.Fatalf("%v: locked, model %v: %v", answers, state, err)
			}
		case SubmitOutcomeUnknown:
			lockLost := submitErr.EndReason == SettleEndNonceUsed || submitErr.EndReason == SettleEndReceiptUnreadable
			if state == stateLocked && !lockLost {
				t.Fatalf("%v: unknown, model locked: %v", answers, err)
			}
		}
	}
}

func TestNotFoundAfterProbesFollowsFourDeliveredSends(t *testing.T) {
	f := fixtures(t)
	r := rand.New(rand.NewPCG(25, 26))
	for range 64 {
		clean := r.IntN(2) == 0
		throttled := make([]bool, 1+r.IntN(7))
		for i := range throttled {
			// A clean start that is throttled at once is rejected; a later
			// throttled send is not.
			throttled[i] = r.IntN(2) == 0 && (i > 0 || !clean)
		}
		s := newScript()
		for _, th := range throttled {
			if th {
				s.withSends(retryLaterAnswer())
			} else {
				s.withSends(unknownAnswer())
			}
		}
		start := startSent
		if clean {
			start = startClean
		}
		err := s.fail(t, f.payment, start, withDeadline(900))
		assertEnd(t, err, SettleEndNotFoundAfterProbes)
		sends, delivered := len(s.times(callSend)), 0
		for index := range sends {
			if index >= len(throttled) || !throttled[index] {
				delivered++
			}
		}
		if delivered != 4 {
			t.Fatalf("%d sends, throttled %v", sends, throttled)
		}
	}
}
