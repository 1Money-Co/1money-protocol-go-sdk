package onemoney

import (
	"encoding/json"
	"fmt"
)

// CheckpointNumber contains the latest checkpoint index.
type CheckpointNumber struct {
	Number int `json:"number"`
}

// checkpointOptions controls optional parameters for checkpoint queries.
type checkpointOptions struct {
	full bool
}

// CheckpointOption mutates checkpointOptions; use helpers such as WithFullTransactions.
type CheckpointOption func(*checkpointOptions)

// WithFullTransactions instructs checkpoint queries to return full transaction objects
// instead of only transaction hashes.
func WithFullTransactions() CheckpointOption {
	return func(opts *checkpointOptions) {
		opts.full = true
	}
}

// TokenData captures token metadata included in some checkpoint responses.
type TokenData struct {
	Decimals        string `json:"decimals"`
	MasterAuthority string `json:"master_authority"`
	Symbol          string `json:"symbol"`
}

// CheckpointTransactions holds either transaction hashes or full transaction objects.
type CheckpointTransactions struct {
	Hashes []string
	Full   []Transaction
}

// UnmarshalJSON accepts both string slices (hashes) and full transaction slices.
func (ct *CheckpointTransactions) UnmarshalJSON(data []byte) error {
	var hashes []string
	if err := json.Unmarshal(data, &hashes); err == nil {
		ct.Hashes = hashes
		ct.Full = nil
		return nil
	}

	var full []Transaction
	if err := json.Unmarshal(data, &full); err == nil {
		ct.Full = full
		ct.Hashes = nil
		return nil
	}

	return fmt.Errorf("transactions must be either string array or Transaction array")
}

// MarshalJSON emits hashes when available, otherwise full transaction objects.
func (ct CheckpointTransactions) MarshalJSON() ([]byte, error) {
	if ct.Full != nil {
		return json.Marshal(ct.Full)
	}
	return json.Marshal(ct.Hashes)
}

// Checkpoint models a checkpoint response with flexible transaction data.
type Checkpoint struct {
	Hash             string                 `json:"hash"`              // The checkpoint hash
	Number           uint64                 `json:"number"`            // The checkpoint number
	ParentHash       string                 `json:"parent_hash"`       // The parent checkpoint hash
	TransactionsRoot string                 `json:"transactions_root"` // Transactions root hash
	ReceiptsRoot     string                 `json:"receipts_root"`     // Transactions receipts root hash
	StateRoot        string                 `json:"state_root"`        // State root hash
	Timestamp        uint64                 `json:"timestamp"`
	Size             int                    `json:"size"`         // Integer the size of this checkpoint in bytes.
	Transactions     CheckpointTransactions `json:"transactions"` // Checkpoint transactions.
	ExtraData        string                 `json:"extra_data"`
}
