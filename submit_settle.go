package onemoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// The node's retry gate forwards one resend of a locked transaction per gap:
// its 60 s duplicate cache plus its 10 s publish queue.
const (
	defaultResendInterval  = 70 * time.Second
	defaultFirstReadDelay  = 500 * time.Millisecond
	defaultPollInterval    = 500 * time.Millisecond
	defaultMaxPollInterval = 10 * time.Second
	defaultSettleDeadline  = 15 * time.Minute
	defaultThrottleBackoff = 60 * time.Second
	// One resend interval: a node that is behind can serve no receipt for that
	// long.
	defaultNonceUsedGrace = defaultResendInterval
	// Lookups answered 404 in a row that end a run early: most likely from
	// several nodes behind a load balancer.
	missesToEnd     = 3
	maxObservations = 200
	acceptedMessage = "the node admitted the transaction"
)

// probeGaps: while no node is known to hold the transaction, a lookup that
// answers 404 starts a send at once, because the node's retry gate forwards a
// hash that it does not hold. Probe k waits probeGaps[k] after the last send
// that may have reached a node; the first second covers the time between a
// 2xx and the node's shared lock.
var probeGaps = [...]time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// SettleOptions is the schedule of a settle run. Start from
// DefaultSettleOptions: every duration must be greater than zero. A much
// shorter PollInterval or ResendInterval loads the node and can get the
// requests throttled.
type SettleOptions struct {
	// ResendInterval: a resend goes out this long after the last resend that
	// may have reached a node, or after the transaction became locked.
	ResendInterval time.Duration
	// FirstReadDelay is the wait between the first send and the first receipt
	// read.
	FirstReadDelay time.Duration
	// PollInterval is the first wait between two reads. It doubles after each
	// read, up to MaxPollInterval. A 404 that a run counts waits this long
	// and does not double, until the first throttled read.
	PollInterval time.Duration
	// MaxPollInterval is not less than PollInterval.
	MaxPollInterval time.Duration
	// Deadline: the run ends with an error this long after its start.
	Deadline time.Duration
	// ThrottleBackoff is the wait after a read or a send answered 429 or
	// retry_later without a positive retry_after_secs. A hint from the node
	// replaces it. No send goes out during the wait.
	ThrottleBackoff time.Duration
	// NonceUsedGrace: after a send answered nonce_too_low, the longest wait for
	// the receipt while the receipt reads keep failing. Three receipt reads in
	// a row that answer 404 end the run sooner. A settled authority operation
	// and a lane clawback do not use it: their runs read until the deadline.
	NonceUsedGrace time.Duration
}

// DefaultSettleOptions returns the schedule that matches the node.
func DefaultSettleOptions() SettleOptions {
	return SettleOptions{
		ResendInterval:  defaultResendInterval,
		FirstReadDelay:  defaultFirstReadDelay,
		PollInterval:    defaultPollInterval,
		MaxPollInterval: defaultMaxPollInterval,
		Deadline:        defaultSettleDeadline,
		ThrottleBackoff: defaultThrottleBackoff,
		NonceUsedGrace:  defaultNonceUsedGrace,
	}
}

func (o SettleOptions) validate() error {
	for _, option := range []struct {
		name  string
		value time.Duration
	}{
		{"ResendInterval", o.ResendInterval},
		{"FirstReadDelay", o.FirstReadDelay},
		{"PollInterval", o.PollInterval},
		{"MaxPollInterval", o.MaxPollInterval},
		{"Deadline", o.Deadline},
		{"ThrottleBackoff", o.ThrottleBackoff},
		{"NonceUsedGrace", o.NonceUsedGrace},
	} {
		if option.value <= 0 {
			return fmt.Errorf("settle option %s must be greater than zero", option.name)
		}
	}
	if o.MaxPollInterval < o.PollInterval {
		return errors.New("settle option MaxPollInterval must not be less than PollInterval")
	}
	return nil
}

// SettledTransaction is a transaction that executed.
type SettledTransaction struct {
	TransactionHash common.Hash
	// Receipt: read Success. A hold debit (reserve claim, clawback from a
	// hold) can execute and fail.
	Receipt *TransactionReceiptResponse
}

// SettleEndReason is why a settle run ended without a receipt that this SDK
// can read.
type SettleEndReason string

const (
	// SettleEndDeadline: SettleOptions.Deadline passed.
	SettleEndDeadline SettleEndReason = "deadline"
	// SettleEndNonceUsed: a send answered nonce_too_low, and the receipt did
	// not appear: three receipt reads in a row answered 404, or
	// SettleOptions.NonceUsedGrace passed. Never for a settled authority
	// operation or a lane clawback.
	SettleEndNonceUsed SettleEndReason = "nonce_used"
	// SettleEndSettling: a settled authority operation or a lane clawback
	// whose send answered nonce_too_low, and which a node reported
	// counter-signed, showed no receipt until the deadline. It is final: read
	// the receipt again later; do not sign the intent again.
	SettleEndSettling SettleEndReason = "settling"
	// SettleEndNonceUsedOutcomeUnknown: a settled authority operation or a
	// lane clawback whose send answered nonce_too_low showed no receipt and no
	// counter-signed status until the deadline. Its nonce is used, by it or by
	// another transaction, and the reads do not tell which: a node
	// bootstrapped from a snapshot, or one that pruned receipts, answers the
	// same for a transaction that executed. Do not sign the intent again until
	// a node with the full history shows no receipt for its hash.
	SettleEndNonceUsedOutcomeUnknown SettleEndReason = "nonce_used_outcome_unknown"
	// SettleEndNotFoundAfterProbes: after the probes, three lookups in a row
	// answered 404. No node that they reached held the transaction.
	SettleEndNotFoundAfterProbes SettleEndReason = "not_found_after_probes"
	// SettleEndCancelled: the context ended.
	SettleEndCancelled SettleEndReason = "cancelled"
	// SettleEndReceiptUnreadable: a node returned the receipt, but this SDK
	// cannot decode it, for example a newer node's receipt. The transaction
	// executed: upgrade the SDK to read it; do not sign the intent again.
	SettleEndReceiptUnreadable SettleEndReason = "receipt_unreadable"
)

// settleRequests are the engine's requests, so that tests can script them.
// An error is a local failure before a request is sent.
type settleRequests interface {
	send(ctx context.Context, authorized *AuthorizedTransaction) (submitOutcome, error)
	receipt(ctx context.Context, hash common.Hash) (readResult[*TransactionReceiptResponse], error)
	lookup(ctx context.Context, hash common.Hash) (readResult[struct{}], error)
	status(ctx context.Context, hash common.Hash) (readResult[TransactionStatus], error)
}

type settleClock interface {
	now() time.Time
	// sleepUntil returns false when ctx ended before at.
	sleepUntil(ctx context.Context, at time.Time) bool
}

type wallClock struct{}

func (wallClock) now() time.Time { return time.Now() }

func (wallClock) sleepUntil(ctx context.Context, at time.Time) bool {
	if ctx.Err() != nil {
		return false
	}
	timer := time.NewTimer(time.Until(at))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type settleStart int

const (
	// startClean sends first.
	startClean settleStart = iota
	// startSent reads first: the transaction was sent before.
	startSent
)

type readPace int

const (
	paceNormal readPace = iota
	// paceMissed: a 404 that a run counts, a lookup while no node is known to
	// hold the transaction or a receipt read after nonce_too_low.
	paceMissed
	// paceThrottled: 429 or retry_later.
	paceThrottled
)

type pace struct {
	kind readPace
	// retryAfter is the node's retry_after_secs, when it sent a positive one.
	retryAfter time.Duration
}

func paceOf[T any](r readResult[T]) pace {
	if r.kind == readThrottled {
		return pace{kind: paceThrottled, retryAfter: retryAfter(r.detail)}
	}
	return pace{kind: paceNormal}
}

// pollSchedule: the waits between reads double from the first wait up to the
// maximum. A throttled read waits the node's hint, else the back-off, and the
// next wait restarts from the maximum. A missed read waits the first wait,
// until the first throttled read: from then on the client stays near the
// firewall's read limit.
type pollSchedule struct {
	nextWait, min, max, throttleBackoff time.Duration
	throttledOnce                       bool
}

func newPollSchedule(o SettleOptions) pollSchedule {
	return pollSchedule{nextWait: o.PollInterval, min: o.PollInterval, max: o.MaxPollInterval, throttleBackoff: o.ThrottleBackoff}
}

func (s *pollSchedule) afterRead(p pace) time.Duration {
	switch {
	case p.kind == paceThrottled:
		s.throttledOnce = true
		s.nextWait = s.max
		wait := s.throttleBackoff
		if p.retryAfter > 0 {
			wait = p.retryAfter
		}
		return max(wait, s.min)
	case p.kind == paceMissed && !s.throttledOnce:
		return s.min
	}
	wait := s.nextWait
	s.nextWait = min(2*wait, s.max)
	return wait
}

func (s *pollSchedule) restart() { s.nextWait = s.min }

// settleState is what the run knows. Locked never goes back: a later answer
// does not undo that proof.
type settleState int

const (
	stateUnknown settleState = iota
	// stateAccepted: a node answered 2xx, but its core can still drop the
	// transaction.
	stateAccepted
	// stateLocked: a lookup found the transaction, or a node said it holds it.
	stateLocked
)

type sendKind int

const (
	// sendFirst: a send of SubmitAndSettle before one may have reached a node.
	sendFirst sendKind = iota
	// sendGate: a resend every ResendInterval.
	sendGate
	// sendProbe: a send after a lookup answered 404; the retry gate does not
	// count it.
	sendProbe
)

type settleRun struct {
	ctx        context.Context
	requests   settleRequests
	clock      settleClock
	authorized *AuthorizedTransaction
	hash       common.Hash
	options    SettleOptions
	schedule   pollSchedule
	endAt      time.Time
	state      settleState
	// gateSent: a first send or a resend may have reached a node. Before one,
	// Settle's first send is due at once; after it, a resend is due
	// ResendInterval after lastGateSend. Probes do not count.
	gateSent bool
	// lastGateSend is the last first send or resend that may have reached a
	// node, or the moment the transaction became locked.
	lastGateSend time.Time
	// sendHoldUntil: no send before it, because a send was throttled.
	sendHoldUntil time.Time
	// lastSend is the last send that may have reached a node.
	lastSend time.Time
	probes   int
	// misses are 404 reads in a row since the last send, toward an early end:
	// lookups after the probes, receipt reads after nonce_too_low.
	misses int
	// nonceUsed: a send answered nonce_too_low. No send can help, and only
	// the receipt can tell whether this transaction used the nonce.
	nonceUsed bool
	// certified: after nonce_too_low on a transaction judged only on
	// evidence, a status lookup reported it counter-signed.
	certified bool
	// lastAnswer is the answer to the last send, or the unreadable receipt.
	lastAnswer *answerDetail
	log        observationLog
}

// runSettle runs the engine for authorized. Requests are not stopped by the
// end of ctx: a send in flight can still reach the node, so the run reads its
// answer, and ends at its next check.
func runSettle(ctx context.Context, requests settleRequests, clock settleClock, authorized *AuthorizedTransaction, start settleStart, options SettleOptions) (*SettledTransaction, error) {
	if authorized == nil {
		return nil, errors.New("nil authorized transaction")
	}
	if err := options.validate(); err != nil {
		return nil, err
	}
	started := clock.now()
	run := &settleRun{
		ctx: ctx, requests: requests, clock: clock, authorized: authorized, hash: common.BytesToHash(authorized.txHash),
		options: options, schedule: newPollSchedule(options), endAt: started.Add(options.Deadline),
		lastGateSend: started, sendHoldUntil: started, lastSend: started,
		log: observationLog{started: started},
	}
	if start == startClean {
		if err := run.firstSend(); err != nil {
			return nil, err
		}
	}
	return run.poll()
}

func (r *settleRun) cancelled() bool { return r.ctx.Err() != nil }

// requestContext keeps a request in flight after a cancel: the run decides at
// its next check.
func (r *settleRun) requestContext() context.Context { return context.WithoutCancel(r.ctx) }

// concludesOnlyOnEvidence: a used nonce without a receipt says nothing about
// a settled authority operation or a lane clawback. An authority operation's
// receipt appears only when its checkpoint commits, after the signer's nonce
// has advanced; a lane clawback re-issued on a wrong "slot taken" claws a
// frozen holder twice.
func (r *settleRun) concludesOnlyOnEvidence() bool {
	return r.authorized.op == opSettledAuthority || r.authorized.op == opLaneClawback
}

// firstSend retries explicit no-effect backpressure before the receipt checks.
// Without that proof, even a 429 can follow admission and starts an unknown
// run.
func (r *settleRun) firstSend() error {
	if r.cancelled() {
		return fmt.Errorf("SubmitAndSettle was cancelled before its first send; the transaction was not sent: %w", r.ctx.Err())
	}
	for {
		delivered, err := r.send(sendFirst)
		if err != nil {
			return err
		}
		if delivered {
			break
		}
		if !r.clock.sleepUntil(r.ctx, minTime(r.sendHoldUntil, r.endAt)) {
			return r.nothingDelivered(SettleEndCancelled)
		}
		if !r.clock.now().Before(r.endAt) {
			return r.nothingDelivered(SettleEndDeadline)
		}
	}
	if !r.clock.sleepUntil(r.ctx, minTime(r.clock.now().Add(r.options.FirstReadDelay), r.endAt)) {
		return r.end(SettleEndCancelled)
	}
	return nil
}

func (r *settleRun) poll() (*SettledTransaction, error) {
	for {
		if r.cancelled() {
			return nil, r.end(SettleEndCancelled)
		}
		receipt, err := r.requests.receipt(r.requestContext(), r.hash)
		if err != nil {
			return nil, err
		}
		r.log.addRead(r.clock.now(), ObservedReceipt, receipt.kind, receipt.detail)
		switch receipt.kind {
		case readFound:
			return &SettledTransaction{TransactionHash: r.hash, Receipt: receipt.value}, nil
		case readUnreadable:
			r.lastAnswer = &receipt.detail
			return nil, r.end(SettleEndReceiptUnreadable)
		}
		p := paceOf(receipt)
		// The last read runs at the end time itself: the deadline, or the
		// nonce_too_low cap.
		if !r.clock.now().Before(r.endAt) {
			return nil, r.end(r.reasonAtEndTime())
		}
		probeDue := false
		switch {
		case r.nonceUsed && !r.concludesOnlyOnEvidence():
			if receipt.kind == readNotFound {
				p = pace{kind: paceMissed}
				r.misses++
				if r.misses >= missesToEnd {
					return nil, r.end(SettleEndNonceUsed)
				}
			} else {
				r.misses = 0
			}
		case r.nonceUsed:
			r.misses = 0
			if p.kind != paceThrottled {
				if err := r.readCertification(); err != nil {
					return nil, err
				}
			}
		case r.state != stateLocked:
			if p.kind == paceThrottled {
				r.misses = 0
			} else {
				lookupPace, due, err := r.lookUp()
				if err != nil {
					return nil, err
				}
				p, probeDue = lookupPace, due
			}
		}
		// One send per pass: a due probe and a due gate send are the same
		// request. A read can end after the end time: no send starts then.
		now := r.clock.now()
		gateAt, gateScheduled := r.nextGateSendAt()
		gateDue := gateScheduled && !now.Before(gateAt)
		if (probeDue || gateDue) && !now.Before(r.sendHoldUntil) && !r.nonceUsed && !r.cancelled() && now.Before(r.endAt) {
			// The first delivered send is never a probe: a probe resends
			// after one.
			deliveredBefore := r.gateSent
			kind := sendProbe
			if gateDue {
				kind = sendGate
			}
			delivered, err := r.send(kind)
			if err != nil {
				return nil, err
			}
			if delivered && probeDue && deliveredBefore {
				r.probes++
			}
		}
		// While sends are on hold, a 404 cannot start a probe, so it does not
		// speed up the reads.
		if p.kind == paceMissed && r.isHoldingSends() {
			p = pace{kind: paceNormal}
		}
		if !r.clock.sleepUntil(r.ctx, r.wakeAt(r.schedule.afterRead(p))) {
			return nil, r.end(SettleEndCancelled)
		}
	}
}

// lookUp looks the transaction up while no node is known to hold it. It
// returns the pace of the next wait, and whether a probe is due.
func (r *settleRun) lookUp() (pace, bool, error) {
	started := r.clock.now()
	lookup, err := r.requests.lookup(r.requestContext(), r.hash)
	if err != nil {
		return pace{}, false, err
	}
	r.log.addRead(r.clock.now(), ObservedLookup, lookup.kind, lookup.detail)
	switch lookup.kind {
	case readFound:
		r.lock()
		return pace{kind: paceNormal}, false, nil
	case readNotFound:
		sinceSend := started.Sub(r.lastSend)
		if r.probes < len(probeGaps) {
			return pace{kind: paceMissed}, sinceSend >= probeGaps[r.probes], nil
		}
		if sinceSend >= probeGaps[0] {
			r.misses++
			if r.misses >= missesToEnd {
				return pace{}, false, r.end(SettleEndNotFoundAfterProbes)
			}
		}
		return pace{kind: paceMissed}, false, nil
	}
	r.misses = 0
	return paceOf(lookup), false, nil
}

// readCertification: after nonce_too_low on a transaction judged only on
// evidence, "nonce used, no receipt" cannot tell this transaction from another
// one at the same nonce, nor from one that executed on a node without its
// history. A status that reports it counter-signed proves it final. A 404
// proves nothing, however long it lasts.
func (r *settleRun) readCertification() error {
	if r.certified {
		return nil
	}
	status, err := r.requests.status(r.requestContext(), r.hash)
	if err != nil {
		return err
	}
	r.log.addRead(r.clock.now(), ObservedStatus, status.kind, status.detail)
	if status.kind == readFound && status.value.IsCounterSigned() {
		r.certified = true
	}
	return nil
}

// send sends the signed bytes once and records what the answer proves. It
// returns whether the send may have reached a node. A refusal ends the first
// send, except a 429. A refusal of the signed bytes also ends a run while no
// node is known to have admitted them; after a 2xx or a found lookup it means
// that the nodes disagree, and ends nothing. Backpressure holds sends, but
// only an explicit no-effect action proves that a send did not reach a node.
func (r *settleRun) send(kind sendKind) (bool, error) {
	outcome, err := r.requests.send(r.requestContext(), r.authorized)
	if err != nil {
		return false, err
	}
	now := r.clock.now()
	switch outcome.kind {
	case outcomeRejected:
		r.log.add(now, ObservedSend, ObservedRejected, &outcome.detail)
		firstRefusal := kind == sendFirst && !isRateLimited(outcome.detail)
		if firstRefusal || (outcome.refusal == SubmitRefusalBytes && r.state == stateUnknown) {
			return false, rejectedError(outcome.detail, outcome.refusal)
		}
	case outcomeUnknown:
		r.log.add(now, ObservedSend, ObservedUnknown, &outcome.detail)
	}
	if outcome.kind == outcomeRejected || outcome.kind == outcomeUnknown {
		if hold, throttled := throttleHold(outcome.detail, r.options.ThrottleBackoff); throttled {
			r.sendHoldUntil = now.Add(hold)
			detail := outcome.detail
			r.lastAnswer = &detail
			if outcome.kind == outcomeRejected {
				return false, nil
			}
		}
	}
	r.lastSend = now
	r.misses = 0
	if kind != sendProbe {
		r.gateSent = true
		r.lastGateSend = now
	}
	detail := outcome.detail
	switch outcome.kind {
	case outcomeAccepted:
		r.log.add(now, ObservedSend, ObservedAccepted, nil)
		if err := checkServerHash(r.hash, outcome.serverHash); err != nil {
			return false, err
		}
		if r.state == stateUnknown {
			r.state = stateAccepted
		}
		detail = answerDetail{summary: AnswerSummary{Status: outcome.status, Message: acceptedMessage}}
	case outcomeLocked:
		r.log.add(now, ObservedSend, ObservedLocked, &outcome.detail)
		r.lock()
	case outcomeUnknown:
		if outcome.nonceUsed && !r.nonceUsed {
			r.nonceUsed = true
			r.schedule.restart()
			if !r.concludesOnlyOnEvidence() {
				r.endAt = minTime(r.endAt, now.Add(r.options.NonceUsedGrace))
			}
		}
	}
	r.lastAnswer = &detail
	return true, nil
}

// nothingDelivered ends a clean run whose sends all got explicit no-effect
// backpressure.
func (r *settleRun) nothingDelivered(reason SettleEndReason) error {
	if r.lastAnswer != nil {
		return rejectedError(*r.lastAnswer, SubmitRefusalRequest)
	}
	return r.end(reason)
}

func (r *settleRun) lock() {
	if r.state == stateLocked {
		return
	}
	r.state = stateLocked
	// A node broadcast the transaction when it locked it. Settle keeps its
	// first send at once.
	if r.gateSent {
		r.lastGateSend = r.clock.now()
	}
}

// nextGateSendAt returns false when no send is due again.
func (r *settleRun) nextGateSendAt() (time.Time, bool) {
	if r.nonceUsed {
		return time.Time{}, false
	}
	due := r.lastGateSend
	if r.gateSent {
		due = due.Add(r.options.ResendInterval)
	}
	if r.sendHoldUntil.After(due) {
		due = r.sendHoldUntil
	}
	return due, true
}

func (r *settleRun) isHoldingSends() bool { return r.clock.now().Before(r.sendHoldUntil) }

// wakeAt is cut at the next due send, at the end of a send hold (a probe can
// be waiting) and at the end time, so that neither a long poll nor a back-off
// moves them.
func (r *settleRun) wakeAt(wait time.Duration) time.Time {
	at := minTime(r.clock.now().Add(wait), r.endAt)
	if gateAt, ok := r.nextGateSendAt(); ok {
		at = minTime(at, gateAt)
	}
	if r.isHoldingSends() {
		at = minTime(at, r.sendHoldUntil)
	}
	return at
}

func (r *settleRun) reasonAtEndTime() SettleEndReason {
	switch {
	case r.nonceUsed && r.concludesOnlyOnEvidence() && r.certified:
		return SettleEndSettling
	case r.nonceUsed && r.concludesOnlyOnEvidence():
		return SettleEndNonceUsedOutcomeUnknown
	case r.nonceUsed:
		return SettleEndNonceUsed
	}
	return SettleEndDeadline
}

// end is the error of a run that ends without a readable receipt: what the
// run proved, the last send's answer (or the unreadable receipt) and every
// observation. Accepted ends as unknown: a 2xx does not prove the lock.
func (r *settleRun) end(reason SettleEndReason) error {
	// A receipt exists, or the nonce is used: either way no resend can help,
	// so "locked, resend it" no longer holds.
	stillLocked := reason != SettleEndReceiptUnreadable && reason != SettleEndNonceUsed &&
		reason != SettleEndSettling && reason != SettleEndNonceUsedOutcomeUnknown
	kind := SubmitOutcomeUnknown
	if r.state == stateLocked && stillLocked {
		kind = SubmitLocked
	}
	err := stateError(kind, r.authorized, r.lastAnswer)
	err.Observations, err.EndReason, err.NonceUsed = r.log.take(), reason, r.nonceUsed
	return err
}

// throttleHold is the send hold after a send answered 429 or retry_later: the
// node's retry_after_secs, else backoff.
func throttleHold(detail answerDetail, backoff time.Duration) (time.Duration, bool) {
	if !isThrottled(detail) {
		return 0, false
	}
	if hint := retryAfter(detail); hint > 0 {
		return hint, true
	}
	return backoff, true
}

// retryAfter is the node's retry_after_secs when it is positive, else 0.
func retryAfter(detail answerDetail) time.Duration {
	if detail.apiErr == nil || detail.apiErr.Type == "" {
		return 0
	}
	secs, ok := detail.apiErr.RetryAfterSecs()
	if !ok || secs == 0 {
		return 0
	}
	// The deadline cuts every wait, so a hint that overflows a Duration waits
	// as long as the longest one.
	return time.Duration(min(secs, uint64(math.MaxInt64/int64(time.Second)))) * time.Second
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// SubmitAndSettle sends a transaction, then resends the same bytes on the
// node's schedule until its receipt appears. The first send is read as Submit
// reads it: only a rejection ends the run at once, except a 429, which holds
// the sends for the back-off. A rejection says only that this send had no
// effect: bytes that were sent before, for example by a call that did not
// finish before a restart, go to Settle.
//
// Errors: a *SubmitError of kind SubmitRejected when the node rejects the
// first send to reach it, when the run ends while every send so far got a 429
// with a no-effect action, or, as in Settle, for a refusal of the signed
// bytes. SubmitLocked or SubmitOutcomeUnknown, with the observations and the
// EndReason, when the run ends without a receipt; the deadline is the longest
// run, not the usual one. Call Settle again later with the same bytes; do not
// sign the intent again. SubmitHashMismatch when the node admits another
// hash. A plain error for an option that is not valid and for a ctx that
// ended before the first send. The end of ctx ends the run at its next check.
func (c *Client) SubmitAndSettle(ctx context.Context, authorized *AuthorizedTransaction, options SettleOptions) (*SettledTransaction, error) {
	return runSettle(ctx, clientRequests{c}, wallClock{}, authorized, startClean, options)
}

// Settle finds the state of a transaction that was sent before, and resends
// the same bytes on the node's schedule until its receipt appears. The first
// receipt read is at once.
//
// Errors: as SubmitAndSettle, but SubmitRejected only for a refusal of the
// signed bytes themselves (signature, chain ID, sender, memo) while no node is
// known to have admitted them: every node refuses such bytes. Any other
// refusal of a resend says nothing about the earlier send, and the run goes
// on.
func (c *Client) Settle(ctx context.Context, authorized *AuthorizedTransaction, options SettleOptions) (*SettledTransaction, error) {
	return runSettle(ctx, clientRequests{c}, wallClock{}, authorized, startSent, options)
}

type clientRequests struct{ c *Client }

func (r clientRequests) send(ctx context.Context, authorized *AuthorizedTransaction) (submitOutcome, error) {
	return r.c.sendAuthorized(ctx, authorized)
}

func (r clientRequests) receipt(ctx context.Context, hash common.Hash) (readResult[*TransactionReceiptResponse], error) {
	got, err := r.c.getAnswer(ctx, hashPath(endpointTransactionsReceiptByHashV1, hash))
	if err != nil {
		return readResult[*TransactionReceiptResponse]{}, err
	}
	return readReceipt(got, hash), nil
}

func (r clientRequests) lookup(ctx context.Context, hash common.Hash) (readResult[struct{}], error) {
	got, err := r.c.getAnswer(ctx, hashPath(endpointTransactionsByHashV1, hash))
	if err != nil {
		return readResult[struct{}]{}, err
	}
	return readTransaction(got, hash), nil
}

func (r clientRequests) status(ctx context.Context, hash common.Hash) (readResult[TransactionStatus], error) {
	got, err := r.c.getAnswer(ctx, hashPath(endpointTransactionsStatusByHashV1, hash))
	if err != nil {
		return readResult[TransactionStatus]{}, err
	}
	return readStatus(got, hash), nil
}

func hashPath(endpoint string, hash common.Hash) string { return endpoint + "?hash=" + hash.Hex() }

// ObservedRequest is the request that a SettleObservation records.
type ObservedRequest string

const (
	// ObservedSend: a send of the signed transaction.
	ObservedSend ObservedRequest = "send"
	// ObservedReceipt: a receipt read by hash.
	ObservedReceipt ObservedRequest = "receipt"
	// ObservedLookup: a transaction lookup by hash.
	ObservedLookup ObservedRequest = "lookup"
	// ObservedStatus: a status lookup by hash.
	ObservedStatus ObservedRequest = "status"
)

// ObservedResult is what one request of a settle run showed.
type ObservedResult string

const (
	// ObservedAccepted: a send that the node answered with the local hash.
	// The node can still drop the transaction before its shared lock.
	ObservedAccepted ObservedResult = "accepted"
	// ObservedRejected: a send refused before admission. Unless it ends the
	// run, it says nothing about the earlier sends.
	ObservedRejected ObservedResult = "rejected"
	// ObservedLocked: a send answered that the node holds this transaction.
	ObservedLocked ObservedResult = "locked"
	// ObservedUnknown: a send whose answer does not tell.
	ObservedUnknown ObservedResult = "unknown"
	// ObservedFound: a read that found the receipt or the transaction.
	ObservedFound ObservedResult = "found"
	// ObservedNotFound: a read answered 404. It proves nothing: the node can
	// be behind.
	ObservedNotFound ObservedResult = "not_found"
	// ObservedThrottled: a read answered 429 or retry_later.
	ObservedThrottled ObservedResult = "throttled"
	// ObservedFailed: a read with any other answer, or no answer.
	ObservedFailed ObservedResult = "failed"
)

// SettleObservation is one request of a settle run and what it showed.
type SettleObservation struct {
	// Seq starts at 1. A gap at the start of the list shows how many of the
	// oldest entries were dropped.
	Seq uint32
	// Elapsed is measured from the start of the run.
	Elapsed time.Duration
	Request ObservedRequest
	Result  ObservedResult
	// Status is 0 when no HTTP answer arrived.
	Status int
	// Action is the body's action member as sent, only in a body whose type
	// is the node's or about:blank.
	Action json.RawMessage
	// ProblemType is the problem type URI, when the body has one.
	ProblemType string
	// Message is the node's text of a send answer, for people to read. Empty
	// for reads and for an accepted send.
	Message string
}

type observationLog struct {
	started time.Time
	seq     uint32
	entries []SettleObservation
}

func (l *observationLog) add(now time.Time, request ObservedRequest, result ObservedResult, detail *answerDetail) {
	l.seq++
	entry := SettleObservation{Seq: l.seq, Elapsed: now.Sub(l.started), Request: request, Result: result}
	if detail != nil {
		entry.Status, entry.Action = detail.summary.Status, detail.action
		if detail.apiErr != nil {
			entry.ProblemType = detail.apiErr.Type
		}
		if request == ObservedSend {
			entry.Message = detail.summary.Message
		}
	}
	l.entries = append(l.entries, entry)
	if len(l.entries) > maxObservations {
		l.entries = l.entries[1:]
	}
}

func (l *observationLog) addRead(now time.Time, request ObservedRequest, kind readKind, detail answerDetail) {
	switch kind {
	case readFound:
		l.add(now, request, ObservedFound, nil)
	case readUnreadable:
		l.add(now, request, ObservedFound, &detail)
	case readNotFound:
		l.add(now, request, ObservedNotFound, &detail)
	case readThrottled:
		l.add(now, request, ObservedThrottled, &detail)
	default:
		l.add(now, request, ObservedFailed, &detail)
	}
}

func (l *observationLog) take() []SettleObservation {
	entries := l.entries
	l.entries = nil
	return entries
}
