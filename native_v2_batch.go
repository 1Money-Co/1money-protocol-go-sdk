package onemoney

import (
	"fmt"

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

// validateBatchOperationAmounts applies the U256 bounds the submit path applies,
// so the exported derivation rejects exactly what signing would reject.
func validateBatchOperationAmounts(operations []PaymentOperation) error {
	for index, operation := range operations {
		if err := validateU256(fmt.Sprintf("batch.operations[%d].amount", index), operation.Amount); err != nil {
			return err
		}
	}
	return nil
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
	encoded, err := rlp.EncodeToBytes(batchOperationsRLPList(operations))
	if err != nil {
		return common.Hash{}, fmt.Errorf("rlp encode batch operations: %w", err)
	}
	return common.BytesToHash(crypto.Keccak256(encoded)), nil
}
