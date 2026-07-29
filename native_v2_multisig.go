package onemoney

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// dstMultisigAddrV1 is the domain-separation tag for multisig address
// derivation (L1 DST_MULTISIG_ADDR_V1).
const dstMultisigAddrV1 = "MULTISIG_V1"

// maxMultisigTotalWeight is the node's u16 ceiling for the summed signer weight
// (om-primitives MultiSigAccountV1::validate uses checked u16 addition).
const maxMultisigTotalWeight = 0xFFFF

// validateCompressedPubkey verifies pk is a canonical 33-byte SEC1-compressed
// secp256k1 public key: a valid 0x02/0x03 prefix, an on-curve point, and the
// canonical encoding. The node parses every signer key via
// Secp256k1PublicKey::from_sec1_bytes when constructing the CreateMultiSig
// transaction and rejects anything else, so a merely-33-byte blob would derive
// an address to which funds could be irrecoverably sent.
func validateCompressedPubkey(pk []byte) error {
	if len(pk) != 33 {
		return fmt.Errorf("public key must be 33 bytes, got %d", len(pk))
	}
	pub, err := crypto.DecompressPubkey(pk)
	if err != nil {
		return fmt.Errorf("invalid compressed secp256k1 public key: %w", err)
	}
	if !bytes.Equal(crypto.CompressPubkey(pub), pk) {
		return fmt.Errorf("public key is not in canonical SEC1-compressed form")
	}
	return nil
}

// validateMultisigConfig enforces the same rules the node applies when creating a
// multisig account (om-primitives MultiSigAccountV1::validate plus the
// CreateMultiSig envelope pubkey parse): non-empty signers; every public key a
// valid, non-duplicate, canonical compressed secp256k1 key; non-zero weights; a
// u16 total weight that does not overflow; and a threshold in [1, total_weight].
// An address is only meaningful for a configuration the node will accept.
func validateMultisigConfig(signers []MultiSigSigner, threshold uint16) error {
	if len(signers) == 0 {
		return fmt.Errorf("multisig: signers must not be empty")
	}
	var totalWeight uint32
	seen := make(map[string]struct{}, len(signers))
	for i, s := range signers {
		if err := validateCompressedPubkey(s.PublicKey); err != nil {
			return fmt.Errorf("multisig: signer %d: %w", i, err)
		}
		if s.Weight == 0 {
			return fmt.Errorf("multisig: signer %d weight must be greater than 0", i)
		}
		if _, dup := seen[string(s.PublicKey)]; dup {
			return fmt.Errorf("multisig: duplicate signer public key at index %d", i)
		}
		seen[string(s.PublicKey)] = struct{}{}
		totalWeight += uint32(s.Weight)
		if totalWeight > maxMultisigTotalWeight {
			return fmt.Errorf("multisig: total signer weight overflows u16")
		}
	}
	if threshold == 0 || uint32(threshold) > totalWeight {
		return fmt.Errorf("multisig: threshold %d must be in [1, total weight %d]", threshold, totalWeight)
	}
	return nil
}

// DeriveMultisigAddress computes the account address for a multisig
// configuration. It is byte-for-byte identical to the address the 1Money node
// assigns at execution (om-primitives derive_multisig_address) and is a pure
// function of the signer set and threshold, so callers can compute it BEFORE
// submitting a CreateMultisig transaction (e.g. to pre-fund or display it).
//
// The configuration is validated against the node's rules first
// (validateMultisigConfig): an address is only returned for a configuration the
// node will actually accept, so funds are never pre-sent to an unusable address.
// Each signer's PublicKey must be a canonical 33-byte SEC1-compressed public
// key. The address is keccak256("MULTISIG_V1" || sorted(pubkey||weight) ||
// threshold_be) truncated to the last 20 bytes; signers are sorted ascending by
// compressed public key, so input order does not affect the result.
func DeriveMultisigAddress(signers []MultiSigSigner, threshold uint16) (common.Address, error) {
	if err := validateMultisigConfig(signers, threshold); err != nil {
		return common.Address{}, err
	}
	return deriveMultisigAddressUnchecked(signers, threshold), nil
}

// deriveMultisigAddressUnchecked computes the multisig address without
// re-validating the configuration. It is used by callers that have already
// validated (the public DeriveMultisigAddress) or that validate downstream (the
// submit pipeline validates via resolvePayloadOp before signing or any network
// I/O), so the configuration is checked exactly once per code path.
func deriveMultisigAddressUnchecked(signers []MultiSigSigner, threshold uint16) common.Address {
	sorted := make([]MultiSigSigner, len(signers))
	copy(sorted, signers)
	sort.SliceStable(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].PublicKey, sorted[j].PublicKey) < 0
	})

	data := make([]byte, 0, len(dstMultisigAddrV1)+len(sorted)*34+2)
	data = append(data, dstMultisigAddrV1...)
	for _, s := range sorted {
		data = append(data, s.PublicKey...)
		data = append(data, s.Weight)
	}
	data = append(data, byte(threshold>>8), byte(threshold)) // u16 big-endian

	hash := crypto.Keccak256(data)
	return common.BytesToAddress(hash[12:32])
}
