package onemoney

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// batchOperationsRLPList encodes a batch payment's operations as the canonical
// RLP element list: one [recipient, amount] pair per operation
// (native-v2-signing-spec §4.2). It is the single source of the operation
// encoding, shared by BatchPaymentPayload.rlpList (the signed preimage) and
// DeriveBatchPaymentOperationsHash, so the exported derivation can never drift
// from what the submit path signs. A nil Amount encodes as U256 zero.
func batchOperationsRLPList(operations []PaymentOperation) []interface{} {
	out := make([]interface{}, 0, len(operations))
	for _, operation := range operations {
		out = append(out, []interface{}{operation.Recipient, bigOrZero(operation.Amount)})
	}
	return out
}

// batchOperationsWireList renders a batch payment's operations as JSON body
// elements. Amounts become quoted decimal strings because marshalling *big.Int
// directly emits a bare JSON number. Shared by BatchPaymentPayload.wireFields
// (the v2 submit body) and BatchPaymentFeeEstimateRequest.MarshalJSON, so both
// requests carry one amount representation.
func batchOperationsWireList(operations []PaymentOperation) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(operations))
	for _, operation := range operations {
		out = append(out, map[string]interface{}{
			"recipient": operation.Recipient,
			"amount":    bigStr(operation.Amount),
		})
	}
	return out
}

// validateBatchOperationAmounts applies the U256 encodability bounds only: every
// non-nil amount must be non-negative and representable in 256 bits. It is the
// *encoding* gate, used by paths that must produce correct bytes for whatever
// the caller supplied -- the pure operations-hash derivation and the wire
// marshaller. It deliberately does NOT enforce the node's admission rules; see
// validateBatchPaymentSubmission for those.
func validateBatchOperationAmounts(operations []PaymentOperation) error {
	for index, operation := range operations {
		if err := validateU256(fmt.Sprintf("batch.operations[%d].amount", index), operation.Amount); err != nil {
			return err
		}
	}
	return nil
}

// validateBatchOperationItems applies the first phase of the node's static,
// governance-independent operation rules: a non-empty list followed by a full
// recipient/amount scan. Total overflow is deliberately a separate final phase
// in validateBatchPaymentSubmission, after operations_hash, matching the node.
//
// These are the rules the node applies without consulting the governance
// certificate, so the SDK can apply them offline and fail before signing rather
// than let a caller sign a transaction that is certain to be rejected. The
// node's remaining checks -- batch payments enabled, the configured
// operations-per-batch limit, the encoded-size limit, and fee-asset matching --
// are governance-dependent and are deliberately left to the server; the SDK
// would have to guess at governance state to duplicate them.
//
// A nil amount encodes as U256 zero everywhere in this SDK, so it fails the
// strictly-positive rule here exactly as an explicit zero does.
func validateBatchOperationItems(operations []PaymentOperation) error {
	if len(operations) == 0 {
		return fmt.Errorf("batch payment operations must not be empty")
	}
	// Keep this helper self-contained for direct tests and future callers. On the
	// public prepare path the earlier encoding gate has already checked the same
	// range rule.
	if err := validateBatchOperationAmounts(operations); err != nil {
		return err
	}
	for index, operation := range operations {
		if operation.Recipient == (common.Address{}) {
			return fmt.Errorf("batch payment operation %d has an invalid recipient: the zero address", index)
		}
		amount := bigOrZero(operation.Amount)
		if amount.Sign() == 0 {
			return fmt.Errorf("batch payment operation %d amount must be greater than 0", index)
		}
	}
	return nil
}

// validateBatchOperationTotal performs the node's final static BatchPayment
// phase. The verifier folds the total only after every recipient/amount and the
// optional operations_hash have been checked, and reports one generic overflow
// error rather than attributing it to an operation.
func validateBatchOperationTotal(operations []PaymentOperation) error {
	total := new(big.Int)
	for _, operation := range operations {
		total.Add(total, bigOrZero(operation.Amount))
		if total.BitLen() > 256 {
			return fmt.Errorf("batch payment total amount overflow")
		}
	}
	return nil
}

// validateBatchPaymentSubmission is the complete pre-signing gate for a batch
// payment: the node's static operation rules plus operations-hash consistency.
//
// The hash check closes the gap that made OperationsHash a trap. The node
// re-derives the canonical hash whenever the field is present and rejects a
// mismatch, so a caller who computed it from a stale operation list would
// otherwise sign and submit a transaction that cannot be accepted.
func validateBatchPaymentSubmission(payload BatchPaymentPayload) error {
	if err := validateBatchOperationItems(payload.Operations); err != nil {
		return err
	}
	if payload.OperationsHash != nil {
		// Unchecked: validateBatchOperationItems above already swept the amounts.
		want, err := deriveBatchOperationsHashUnchecked(payload.Operations)
		if err != nil {
			return err
		}
		if *payload.OperationsHash != want {
			return fmt.Errorf(
				"batch payment operations_hash mismatch: payload has %s, operations derive %s; use DeriveBatchPaymentOperationsHash or leave the field nil",
				payload.OperationsHash.Hex(), want.Hex(),
			)
		}
	}
	return validateBatchOperationTotal(payload.Operations)
}

// DeriveBatchPaymentOperationsHash computes the canonical operations hash for a
// batch payment's operation list. It is byte-for-byte identical to the node's
// BatchPaymentPayload::canonical_operations_hash — keccak256 of the RLP-encoded
// operation list — and is a pure function of the operations, so callers can fill
// BatchPaymentPayload.OperationsHash themselves. The node re-derives and compares
// the value whenever the field is present, and rejects the transaction on a
// mismatch.
//
// Leaving OperationsHash nil is always valid; the field exists so a system that
// defines a batch can publish its operation set independently of the system that
// builds and signs the transaction.
//
// A nil PaymentOperation.Amount is treated as U256 zero, matching the submit
// encoder, so the returned hash always covers exactly the bytes the submit path
// would sign. That does not make the operation admission-valid: the node rejects
// batch operations whose amount is zero. Negative amounts and values wider than
// 256 bits return an error. Operation order is significant and is never
// normalized.
func DeriveBatchPaymentOperationsHash(operations []PaymentOperation) (common.Hash, error) {
	if err := validateBatchOperationAmounts(operations); err != nil {
		return common.Hash{}, err
	}
	return deriveBatchOperationsHashUnchecked(operations)
}

// deriveBatchOperationsHashUnchecked hashes the operation list without
// re-validating it, for callers that have already validated. Mirrors the
// checked/unchecked split deriveMultisigAddressUnchecked uses, and keeps the
// amounts from being range-checked twice on the signing path.
func deriveBatchOperationsHashUnchecked(operations []PaymentOperation) (common.Hash, error) {
	encoded, err := rlp.EncodeToBytes(batchOperationsRLPList(operations))
	if err != nil {
		return common.Hash{}, fmt.Errorf("rlp encode batch operations: %w", err)
	}
	return common.BytesToHash(crypto.Keccak256(encoded)), nil
}
