package onemoney

import "github.com/ethereum/go-ethereum/common"

// MultiSigSigner is one member of a multisig account: a 33-byte SEC1-compressed
// public key and a voting weight.
type MultiSigSigner struct {
	PublicKey HexBytes `json:"public_key"`
	Weight    uint8    `json:"weight"`
}

// CreateMultiSigPayload creates a multisig account with the given signer set and
// approval threshold. The creation transaction itself is single-signed.
type CreateMultiSigPayload struct {
	ChainID   uint64           `json:"chain_id"`
	Nonce     uint64           `json:"nonce"`
	Signers   []MultiSigSigner `json:"signers"`
	Threshold uint16           `json:"threshold"`
}

// CreateMultisigResponse is returned by Accounts().CreateMultisig. Hash is the
// submitted transaction hash from the node. Account is the created multisig
// account address; the L1 endpoint returns only the hash, so the SDK fills
// Account by local derivation (deterministic and identical to the address the
// node assigns — see DeriveMultisigAddress).
type CreateMultisigResponse struct {
	Hash    string         `json:"hash"`
	Account common.Address `json:"account"`
}

// TxHash reports the submitted transaction hash for hash-verification.
func (r *CreateMultisigResponse) TxHash() string { return r.Hash }
