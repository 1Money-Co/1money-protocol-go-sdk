package onemoney

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// TransactionStatus is where a transaction stands, as GET
// /v1/transactions/status/by_hash reports it. The node can add values; an
// unknown one is not counter-signed.
type TransactionStatus string

const (
	// TransactionStatusPending: in the node's pool, not counter-signed yet.
	TransactionStatusPending TransactionStatus = "pending"
	// TransactionStatusFinalized: counter-signed and executed.
	TransactionStatusFinalized TransactionStatus = "finalized"
	// TransactionStatusCertifiedPendingCheckpoint: counter-signed, so final,
	// and settling: a settled authority operation (or a Reserved Payment hold
	// debit) has its outcome in its receipt when its checkpoint commits.
	TransactionStatusCertifiedPendingCheckpoint TransactionStatus = "certified_pending_checkpoint"
	// TransactionStatusCheckpointed: in a committed checkpoint.
	TransactionStatusCheckpointed TransactionStatus = "checkpointed"
)

// IsKnown reports whether s is one of the four statuses above.
func (s TransactionStatus) IsKnown() bool {
	switch s {
	case TransactionStatusPending, TransactionStatusFinalized,
		TransactionStatusCertifiedPendingCheckpoint, TransactionStatusCheckpointed:
		return true
	}
	return false
}

// IsCounterSigned reports whether s means the transaction has its 2f+1
// certificate, which is final.
func (s TransactionStatus) IsCounterSigned() bool {
	switch s {
	case TransactionStatusFinalized, TransactionStatusCertifiedPendingCheckpoint, TransactionStatusCheckpointed:
		return true
	}
	return false
}

// TransactionStatusResponse is the answer of GET /v1/transactions/status/by_hash.
type TransactionStatusResponse struct {
	Hash   string            `json:"hash"`
	Status TransactionStatus `json:"status"`
	// CheckpointNumber is set once the transaction is in a committed checkpoint.
	CheckpointNumber *uint64 `json:"checkpoint_number,omitempty"`
}

// GetTransactionStatus reads where a transaction stands. A transaction that
// the node does not know answers HTTP 404 (*APIError).
func (c *Client) GetTransactionStatus(ctx context.Context, hash string) (*TransactionStatusResponse, error) {
	result := new(TransactionStatusResponse)
	params := url.Values{}
	params.Set("hash", hash)
	return result, c.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointTransactionsStatusByHashV1, params.Encode()), result)
}

// Execution event types of a receipt. The node can add types; an unknown one
// is neither a no-op nor a soft fail.
const (
	ExecutionEventNoOp     = "NoOp"
	ExecutionEventSoftFail = "SoftFail"
)

// SoftFailReason is why a settled operation or a hold debit failed at its
// checkpoint. The node can add reasons: an unknown value is kept as sent.
type SoftFailReason string

const (
	SoftFailHoldNotFound       SoftFailReason = "HoldNotFound"
	SoftFailHoldInsufficient   SoftFailReason = "HoldInsufficient"
	SoftFailComplianceRejected SoftFailReason = "ComplianceRejected"
	SoftFailAuthorityChanged   SoftFailReason = "AuthorityChanged"
	SoftFailRoleMissing        SoftFailReason = "RoleMissing"
	SoftFailCapReached         SoftFailReason = "CapReached"
	SoftFailAllowanceShort     SoftFailReason = "AllowanceShort"
	SoftFailRotationConflict   SoftFailReason = "RotationConflict"
	SoftFailRecipientNotWallet SoftFailReason = "RecipientNotWallet"
)

// IsKnown reports whether r is one of the nine reasons above.
func (r SoftFailReason) IsKnown() bool {
	switch r {
	case SoftFailHoldNotFound, SoftFailHoldInsufficient, SoftFailComplianceRejected, SoftFailAuthorityChanged,
		SoftFailRoleMissing, SoftFailCapReached, SoftFailAllowanceShort, SoftFailRotationConflict,
		SoftFailRecipientNotWallet:
		return true
	}
	return false
}

// ReceiptWarningNoOpRevoke: a revoke changed nothing because the named account
// held no role. Another account can still hold the role, for example the
// stolen key that the issuer meant to revoke: show this to the user.
const ReceiptWarningNoOpRevoke = "no_op_revoke"

// ReceiptWarning is one item of a receipt's warnings.
type ReceiptWarning struct {
	Type   string          `json:"type"`
	Target *common.Address `json:"target"`
}

// ReceiptOutcomeKind is the outcome that a receipt decides.
type ReceiptOutcomeKind string

const (
	ReceiptSuccess ReceiptOutcomeKind = "success"
	// ReceiptNoOp: success, and the target state already held.
	ReceiptNoOp ReceiptOutcomeKind = "no_op"
	// ReceiptSoftFail: the checkpoint refused it with a reason. It consumed the
	// signer's nonce and changed nothing else.
	ReceiptSoftFail ReceiptOutcomeKind = "soft_fail"
	ReceiptFailed   ReceiptOutcomeKind = "failed"
)

// ReceiptOutcome is what a receipt says about its transaction.
type ReceiptOutcome struct {
	Kind ReceiptOutcomeKind
	// Target is the NoOp event's target: nil for an operation that names no account.
	Target *common.Address
	// SoftFailReason is set for ReceiptSoftFail, possibly to a value that this
	// SDK does not know (see SoftFailReason.IsKnown); empty when the event has none.
	SoftFailReason SoftFailReason
	// NoOpRevoke is the no_op_revoke warning: show it to the user.
	NoOpRevoke *ReceiptWarning
}

// ReceiptOutcomeOf reads the outcome of a receipt.
func ReceiptOutcomeOf(receipt *TransactionReceiptResponse) ReceiptOutcome {
	if !receipt.Success {
		for _, event := range receipt.ExecutionEvents {
			if event.EventType == ExecutionEventSoftFail {
				outcome := ReceiptOutcome{Kind: ReceiptSoftFail}
				if event.Reason != nil {
					outcome.SoftFailReason = SoftFailReason(*event.Reason)
				}
				return outcome
			}
		}
		return ReceiptOutcome{Kind: ReceiptFailed}
	}
	for _, event := range receipt.ExecutionEvents {
		if event.EventType == ExecutionEventNoOp {
			outcome := ReceiptOutcome{Kind: ReceiptNoOp, Target: event.Target}
			for i := range receipt.Warnings {
				if receipt.Warnings[i].Type == ReceiptWarningNoOpRevoke {
					warning := receipt.Warnings[i]
					outcome.NoOpRevoke = &warning
				}
			}
			return outcome
		}
	}
	return ReceiptOutcome{Kind: ReceiptSuccess}
}

// AuthorityRefusalKind is a refusal of the token authority model: the problem
// type's ID.
type AuthorityRefusalKind string

const (
	// RefusalLegacyEnvelopeRetired: the node no longer takes the legacy
	// envelope of this operation, and took no nonce and no lock for it. Never
	// send the legacy form again; read the envelope mode, and once it is
	// settled, sign the operation again as the new envelope.
	RefusalLegacyEnvelopeRetired AuthorityRefusalKind = "legacy_envelope_retired"
	// RefusalSettledEnvelopeNotActive: the node has not activated the new
	// envelopes yet. The same transaction can succeed later.
	RefusalSettledEnvelopeNotActive AuthorityRefusalKind = "settled_envelope_not_active"
	// RefusalReferenceCheckpointNotCommitted: the node has not committed the
	// reference checkpoint yet; the request had no effect. Send the same signed
	// transaction again after the next checkpoint.
	RefusalReferenceCheckpointNotCommitted AuthorityRefusalKind = "reference_checkpoint_not_committed"
	// RefusalReferenceCheckpointTooOld: the reference is more than 100
	// checkpoints behind, or pruned. A node checks the reference before the
	// nonce, so a transaction that already executed gets this answer too: if it
	// was sent before, read its outcome before you sign again.
	RefusalReferenceCheckpointTooOld AuthorityRefusalKind = "reference_checkpoint_too_old"
	// RefusalTokenNotAtReferenceCheckpoint: the token is not in the state at
	// the reference, for example because its issue is in no committed
	// checkpoint yet; the request had no effect. It does not show that the
	// token does not exist: sign again at the same nonce with a later reference.
	RefusalTokenNotAtReferenceCheckpoint AuthorityRefusalKind = "token_not_at_reference_checkpoint"
)

var authorityRefusalKinds = map[AuthorityRefusalKind]bool{
	RefusalLegacyEnvelopeRetired: true, RefusalSettledEnvelopeNotActive: true,
	RefusalReferenceCheckpointNotCommitted: true, RefusalReferenceCheckpointTooOld: true,
	RefusalTokenNotAtReferenceCheckpoint: true,
}

// AuthorityRefusal is a node's refusal of an authority operation at admission,
// with its typed members.
type AuthorityRefusal struct {
	Kind AuthorityRefusalKind
	// CheckpointID is the reference checkpoint that the request named.
	CheckpointID *uint64
	// CheckpointIDLatest is the node's latest committed checkpoint
	// (RefusalReferenceCheckpointNotCommitted).
	CheckpointIDLatest *uint64
	// CheckpointIDEarliest is the earliest reference that the node accepts
	// (RefusalReferenceCheckpointTooOld).
	CheckpointIDEarliest *uint64
	// MintAddress is the token (RefusalTokenNotAtReferenceCheckpoint).
	MintAddress *common.Address
	Err         *APIError
}

// AuthorityRefusalOf reads err as one of the five refusals of the token
// authority model. Only a 4xx *APIError of a type in this SDK's catalog
// counts; anything else, such as a transport error, returns false.
func AuthorityRefusalOf(err error) (*AuthorityRefusal, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode < 400 || apiErr.StatusCode >= 500 || !apiErr.IsKnownType() {
		return nil, false
	}
	kind := AuthorityRefusalKind(strings.TrimPrefix(apiErr.Type, ProblemTypePrefix))
	if !authorityRefusalKinds[kind] {
		return nil, false
	}
	refusal := &AuthorityRefusal{Kind: kind, Err: apiErr}
	refusal.CheckpointID = uint64Member(apiErr.Problem, "checkpoint_id")
	refusal.CheckpointIDLatest = uint64Member(apiErr.Problem, "checkpoint_id_latest")
	refusal.CheckpointIDEarliest = uint64Member(apiErr.Problem, "checkpoint_id_earliest")
	var mint common.Address
	if apiErr.Member("mint_address", &mint) {
		refusal.MintAddress = &mint
	}
	return refusal, true
}

func uint64Member(p Problem, name string) *uint64 {
	var value uint64
	if p.Member(name, &value) {
		return &value
	}
	return nil
}

func isRefusal(err error, kinds ...AuthorityRefusalKind) bool {
	refusal, ok := AuthorityRefusalOf(err)
	if !ok {
		return false
	}
	for _, kind := range kinds {
		if refusal.Kind == kind {
			return true
		}
	}
	return false
}
