package onemoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// UnknownOutcome is what one round of reads showed about a transaction whose
// account nonce has passed its nonce while no receipt was found and no node
// reported it counter-signed. It does not tell whether the transaction
// executed: a node bootstrapped from a snapshot holds no receipt and no
// certificate before its snapshot, and a node can prune receipts, so such a
// node answers the same for a transaction that executed as for one whose
// nonce another transaction took.
type UnknownOutcome struct {
	// NextNonce is the account's next nonce on the node that answered.
	NextNonce uint64
	// Status is the status by hash; nil when the node answered 404.
	Status *TransactionStatus
}

// AuthorityOperationStateKind is where an authority operation stands.
type AuthorityOperationStateKind string

const (
	// AuthorityPending: the signer's nonce has not passed the operation's
	// nonce, and no receipt exists. A settled operation can stay pending for up
	// to three epoch transitions before the nodes evict it: a long pending is
	// not a failure.
	AuthorityPending AuthorityOperationStateKind = "pending"
	// AuthoritySettling: a node reports the operation counter-signed, and no
	// receipt was found yet. It is final, and its checkpoint decides it. This
	// is not a failure.
	AuthoritySettling AuthorityOperationStateKind = "settling"
	// AuthorityDecided: the receipt decides it; read it with ReceiptOutcomeOf.
	AuthorityDecided AuthorityOperationStateKind = "decided"
	// AuthorityUnknown: the signer's nonce has passed the operation's nonce,
	// with no receipt and no counter-signed status. It may have executed, or
	// another transaction may have taken the nonce. Never report it as failed,
	// and do not sign the intent again until a node with the full history shows
	// no receipt for its hash.
	AuthorityUnknown AuthorityOperationStateKind = "unknown"
)

// AuthorityOperationState is one round of reads about an authority operation.
type AuthorityOperationState struct {
	Kind AuthorityOperationStateKind
	// Receipt is set for AuthorityDecided.
	Receipt *TransactionReceiptResponse
	// Status is the counter-signed status of AuthoritySettling.
	Status *TransactionStatus
	// Unknown is set for AuthorityUnknown.
	Unknown *UnknownOutcome
}

// AuthorityOperationState reads, in one round, where the authority operation
// of signer at nonce stands. Only a receipt (AuthorityDecided) or a status by
// hash that reports it counter-signed (AuthoritySettling) decides it. "Nonce
// passed, no receipt" without such a status is AuthorityUnknown with what the
// reads showed, however often it is read.
//
// Errors: a read that did not answer, a receipt that does not decode, or a
// status that this SDK does not know.
func (c *Client) AuthorityOperationState(ctx context.Context, signer common.Address, nonce uint64, txHash common.Hash) (AuthorityOperationState, error) {
	evidence, err := c.readEvidence(ctx, signer, nonce, txHash)
	if err != nil {
		return AuthorityOperationState{}, err
	}
	switch {
	case evidence.receipt != nil:
		return AuthorityOperationState{Kind: AuthorityDecided, Receipt: evidence.receipt}, nil
	case evidence.certified != nil:
		return AuthorityOperationState{Kind: AuthoritySettling, Status: evidence.certified}, nil
	case evidence.unknown != nil:
		return AuthorityOperationState{Kind: AuthorityUnknown, Unknown: evidence.unknown}, nil
	}
	return AuthorityOperationState{Kind: AuthorityPending}, nil
}

// evidence is one round of reads about a signed transaction at nonce of its
// account. At most one field is set; none means the nonce has not passed.
type evidence struct {
	receipt   *TransactionReceiptResponse
	certified *TransactionStatus
	unknown   *UnknownOutcome
}

// readEvidence reads the account's nonce first, so a transaction that executes
// after that read never reads as not passed: its receipt or its status is read
// after it. The status is read before the nonce decides anything, so a
// certificate wins over a nonce read from a node that is behind.
func (c *Client) readEvidence(ctx context.Context, account common.Address, nonce uint64, txHash common.Hash) (evidence, error) {
	next, err := c.GetAccountNonce(ctx, account)
	if err != nil {
		return evidence{}, fmt.Errorf("nonce read of %s: %w", account.Hex(), err)
	}
	receipt, err := c.GetTransactionReceipt(ctx, txHash.Hex())
	foundReceipt, err := foundOrNone(receipt, err, "receipt read", txHash)
	if err != nil {
		return evidence{}, err
	}
	if foundReceipt != nil {
		if err := namesHash(foundReceipt.TransactionHash, "receipt read", txHash); err != nil {
			return evidence{}, err
		}
		return evidence{receipt: foundReceipt}, nil
	}
	statusResponse, err := c.GetTransactionStatus(ctx, txHash.Hex())
	found, err := foundOrNone(statusResponse, err, "status lookup", txHash)
	if err != nil {
		return evidence{}, err
	}
	var status *TransactionStatus
	if found != nil {
		if err := namesHash(found.Hash, "status lookup", txHash); err != nil {
			return evidence{}, err
		}
		if !found.Status.IsKnown() {
			return evidence{}, fmt.Errorf("the status lookup of %s answered %q, which this SDK does not know", txHash.Hex(), found.Status)
		}
		status = &found.Status
	}
	switch {
	case status != nil && status.IsCounterSigned():
		return evidence{certified: status}, nil
	case next.Nonce <= nonce:
		return evidence{}, nil
	}
	return evidence{unknown: &UnknownOutcome{NextNonce: next.Nonce, Status: status}}, nil
}

// foundOrNone returns the read, or nil for a 404; every other failure is an
// error that names the read. A 404 with the node's retry_later is a throttled
// read, not an absent record.
func foundOrNone[T any](result *T, err error, what string, txHash common.Hash) (*T, error) {
	var apiErr *APIError
	switch {
	case err == nil:
		return result, nil
	case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound && apiErr.Action != ActionRetryLater:
		return nil, nil
	}
	return nil, fmt.Errorf("the %s of %s did not answer: %w", what, txHash.Hex(), err)
}

// namesHash refuses a read that answered 200 for another transaction, or with
// a body that names none (such as {} or null): it is no evidence about txHash.
func namesHash(answered, what string, txHash common.Hash) error {
	if !strings.EqualFold(answered, txHash.Hex()) {
		return fmt.Errorf("the %s of %s answered for %q, not for this transaction", what, txHash.Hex(), answered)
	}
	return nil
}

// LaneClawbackRecord is a signed lane clawback: its payload, memo and hash
// always belong together. It marshals as its signed transaction, and
// ParseLaneClawbackRecord checks the transaction against its hash: keep it
// across a restart this way.
type LaneClawbackRecord struct {
	payload     LaneClawbackPayload
	memo        Memo
	transaction *AuthorizedTransaction
}

// NewLaneClawbackRecord makes the record of a signed lane clawback.
// Errors: a transaction of another operation.
func NewLaneClawbackRecord(transaction *AuthorizedTransaction) (*LaneClawbackRecord, error) {
	if transaction == nil || transaction.op != opLaneClawback {
		return nil, fmt.Errorf("the transaction is not a lane clawback")
	}
	body, err := json.Marshal(transaction.body)
	if err != nil {
		return nil, err
	}
	payload, _, err := decodeLaneClawbackBody(body)
	if err != nil {
		return nil, err
	}
	memo, _ := transaction.body["memo"].(Memo)
	return &LaneClawbackRecord{payload: payload.(LaneClawbackPayload), memo: memo, transaction: transaction}, nil
}

// ParseLaneClawbackRecord loads a record that json.Marshal stored.
// Errors: as ParseAuthorizedTransaction; a transaction of another operation.
func ParseLaneClawbackRecord(data []byte) (*LaneClawbackRecord, error) {
	transaction, err := ParseAuthorizedTransaction(data)
	if err != nil {
		return nil, err
	}
	return NewLaneClawbackRecord(transaction)
}

// MarshalJSON stores the record as its signed transaction.
func (r *LaneClawbackRecord) MarshalJSON() ([]byte, error) { return r.transaction.MarshalJSON() }

// Payload returns the signed payload. Its Value is a copy.
func (r *LaneClawbackRecord) Payload() LaneClawbackPayload {
	payload := r.payload
	payload.Value = new(big.Int).Set(bigOrZero(r.payload.Value))
	return payload
}

func (r *LaneClawbackRecord) Memo() Memo { return r.memo }

func (r *LaneClawbackRecord) TransactionHash() common.Hash {
	return common.BytesToHash(r.transaction.txHash)
}

// Transaction returns the signed transaction that every send of it uses.
func (r *LaneClawbackRecord) Transaction() *AuthorizedTransaction { return r.transaction }

// LaneClawbackFateKind is where a lane clawback stands. Only a receipt or a
// status that reports it counter-signed says that it executes; nothing that
// this SDK reads says that it never will.
type LaneClawbackFateKind string

const (
	// LaneClawbackExecuted: its receipt exists.
	LaneClawbackExecuted LaneClawbackFateKind = "executed"
	// LaneClawbackCertified: a node reports it counter-signed and no receipt
	// was found yet. A lane clawback executes at certification, so it executes:
	// never claw back again for it.
	LaneClawbackCertified LaneClawbackFateKind = "certified"
	// LaneClawbackPending: the holder's nonce has not passed FromNonce, and no
	// receipt was found.
	LaneClawbackPending LaneClawbackFateKind = "pending"
	// LaneClawbackUnknown: the holder's nonce is past FromNonce, with no receipt
	// and no counter-signed status. It may have executed, or another
	// transaction may have taken its slot. Never issue it again by itself: a
	// replacement is an explicit issuer action after a check on a node with the
	// full history.
	LaneClawbackUnknown LaneClawbackFateKind = "unknown"
)

// LaneClawbackFate is one round of reads about a lane clawback.
type LaneClawbackFate struct {
	Kind    LaneClawbackFateKind
	Receipt *TransactionReceiptResponse
	Status  *TransactionStatus
	Unknown *UnknownOutcome
}

// LaneClawbackFate reads, in one round, where a lane clawback stands, from the
// holder's nonce, its receipt and its status by hash, as
// AuthorityOperationState does for the signer's nonce.
//
// Errors: as AuthorityOperationState.
func (c *Client) LaneClawbackFate(ctx context.Context, record *LaneClawbackRecord) (LaneClawbackFate, error) {
	evidence, err := c.readEvidence(ctx, record.payload.Holder, record.payload.FromNonce, record.TransactionHash())
	if err != nil {
		return LaneClawbackFate{}, err
	}
	switch {
	case evidence.receipt != nil:
		return LaneClawbackFate{Kind: LaneClawbackExecuted, Receipt: evidence.receipt}, nil
	case evidence.certified != nil:
		return LaneClawbackFate{Kind: LaneClawbackCertified, Status: evidence.certified}, nil
	case evidence.unknown != nil:
		return LaneClawbackFate{Kind: LaneClawbackUnknown, Unknown: evidence.unknown}, nil
	}
	return LaneClawbackFate{Kind: LaneClawbackPending}, nil
}
