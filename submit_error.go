package onemoney

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// SubmitErrorKind is what a submit that did not end with the transaction
// admitted under its local hash proves. Each kind is a different duty for the
// caller; only SubmitRejected proves that the request had no effect.
type SubmitErrorKind string

const (
	// SubmitRejected: the node refused the request before admission. It had no
	// effect.
	SubmitRejected SubmitErrorKind = "rejected"
	// SubmitLocked: the node holds this exact transaction, admitted and locked,
	// not executed yet. Resend the same signed transaction (Transaction).
	SubmitLocked SubmitErrorKind = "locked"
	// SubmitOutcomeUnknown: the answer does not tell whether the node admitted
	// the transaction. Find its state first: look up the hash, and resend the
	// same signed transaction.
	SubmitOutcomeUnknown SubmitErrorKind = "outcome_unknown"
	// SubmitHashMismatch: the node admitted a transaction under another hash.
	// Send nothing more: a new send would be a second transaction on the same
	// nonce.
	SubmitHashMismatch SubmitErrorKind = "hash_mismatch"
)

// AnswerSummary is what one HTTP answer said, for error text and for callers.
type AnswerSummary struct {
	// Status is 0 when no HTTP answer arrived.
	Status int
	// ErrorCode is the legacy error_code, when the body has one.
	ErrorCode string
	// Message is the legacy message, else the problem title, else the start of
	// the body, else why no answer arrived. For display only: no decision
	// reads it.
	Message string
}

// String gives "(HTTP 422, business_transaction_failed: already inflight)" or
// "(no HTTP answer: ...)".
func (s AnswerSummary) String() string {
	switch {
	case s.Status == 0:
		return fmt.Sprintf("(no HTTP answer: %s)", s.Message)
	case s.ErrorCode == "":
		return fmt.Sprintf("(HTTP %d: %s)", s.Status, s.Message)
	}
	return fmt.Sprintf("(HTTP %d, %s: %s)", s.Status, s.ErrorCode, s.Message)
}

// SubmitError is the error of every /v2 submit (Client.Submit and the
// TokensAPI and TransactionsAPI methods) that did not end with the
// transaction admitted under its local hash. errors.As(err, &apiErr)
// still finds the *APIError of the answer that decided the kind.
type SubmitError struct {
	Kind SubmitErrorKind
	// TransactionHash is the local hash. Zero for SubmitRejected.
	TransactionHash common.Hash
	// Transaction is the signed transaction to resend with Client.Submit: also
	// the one that a helper signed inside the call. Nil for SubmitRejected and
	// SubmitHashMismatch.
	Transaction *AuthorizedTransaction
	// ServerHash is the hash that the node returned, as sent
	// (SubmitHashMismatch only).
	ServerHash string
	// Answer is the answer that decided the kind. Nil for SubmitHashMismatch.
	Answer *AnswerSummary
	// Refusal tells what a SubmitRejected refused. Empty for the other kinds.
	Refusal SubmitRefusal
	// NonceUsed: a node answered that this transaction's nonce is already used
	// (nonce_too_low). An earlier transaction, possibly this one, used it.
	// Never sign again at this nonce; look up this hash before you sign
	// anything new.
	NonceUsed bool

	apiErr *APIError
	cause  error
}

// RequestHadNoEffect is true only for SubmitRejected. Even then, sign a new
// transaction for the same intent only if no other transaction for it is
// locked, executed or in an unknown state: a rejected resend says nothing
// about an earlier send of the same transaction.
func (e *SubmitError) RequestHadNoEffect() bool { return e.Kind == SubmitRejected }

// Problem is the problem details of the deciding answer, or nil when its body
// has no problem type.
func (e *SubmitError) Problem() *Problem {
	if e.apiErr == nil || e.apiErr.Type == "" {
		return nil
	}
	return &e.apiErr.Problem
}

// Unwrap returns the *APIError of the deciding answer, or the transport
// failure when no HTTP answer arrived.
func (e *SubmitError) Unwrap() error {
	if e.apiErr != nil {
		return e.apiErr
	}
	return e.cause
}

func (e *SubmitError) Error() string {
	answer := ""
	if e.Answer != nil {
		answer = " " + e.Answer.String()
	}
	switch e.Kind {
	case SubmitRejected:
		return "transaction rejected before admission" + answer + ". This request did not lock the transaction or use its " +
			"nonce. Fix the cause before you send again. Sign a new transaction for the same intent only if no other " +
			"transaction for it is locked, executed, or in an unknown state."
	case SubmitLocked:
		return fmt.Sprintf("the node holds transaction %s: it is admitted and locked, but not executed yet%s. Resend the "+
			"same signed transaction until it executes. Do not sign a new transaction for the same intent, at any "+
			"nonce, while it is locked.", e.TransactionHash.Hex(), answer)
	case SubmitHashMismatch:
		return fmt.Sprintf("transaction hash mismatch: the transaction was submitted, but the node returned %s while the "+
			"SDK computed %s. Do not send it again: a new send would be a second transaction on the same nonce.",
			e.ServerHash, e.TransactionHash.Hex())
	}
	return fmt.Sprintf("transaction outcome unknown: the SDK cannot confirm whether the node admitted transaction %s%s. "+
		"Find its state first: look up the hash and resend the same signed transaction. Do not sign a new transaction "+
		"for the same intent, at any nonce, until you know its state.", e.TransactionHash.Hex(), answer)
}

func rejectedError(detail answerDetail, refusal SubmitRefusal) *SubmitError {
	summary := detail.summary
	return &SubmitError{Kind: SubmitRejected, Answer: &summary, Refusal: refusal, apiErr: detail.apiErr, cause: detail.cause}
}

// stateError is a SubmitLocked or SubmitOutcomeUnknown error.
func stateError(kind SubmitErrorKind, authorized *AuthorizedTransaction, detail answerDetail) *SubmitError {
	summary := detail.summary
	return &SubmitError{
		Kind: kind, TransactionHash: common.BytesToHash(authorized.txHash), Transaction: authorized,
		Answer: &summary, apiErr: detail.apiErr, cause: detail.cause,
	}
}
