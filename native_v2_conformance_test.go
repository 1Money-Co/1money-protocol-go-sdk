package onemoney

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// This test proves the Go domain-separated signing core is byte-for-byte
// identical to the L1 reference implementation, by recomputing every field of
// every golden vector and asserting equality. The fixture is copied verbatim
// from l1client docs/specs/fixtures/native-v2-signing-vectors.json.

type conformanceFile struct {
	BaseVectors         []conformanceVector `json:"base_vectors"`
	SupplementalVectors []conformanceVector `json:"supplemental_vectors"`
}

type conformanceVector struct {
	Name                   string          `json:"name"`
	OperationType          uint16          `json:"operation_type"`
	AuthorizationKind      int             `json:"authorization_kind"`
	MultisigAccount        *string         `json:"multisig_account"`
	PayloadRLP             string          `json:"payload_rlp"`
	AuthorizationProof     json.RawMessage `json:"authorization_proof"`
	UnsignedTransactionRLP string          `json:"unsigned_transaction_rlp"`
	SigningHash            string          `json:"signing_hash"`
	SignedTransactionRLP   string          `json:"signed_transaction_rlp"`
	TransactionHash        string          `json:"transaction_hash"`
}

type conformanceSig struct {
	R string `json:"r"`
	S string `json:"s"`
	V uint64 `json:"v"`
}

func loadVectors(t *testing.T) []conformanceVector {
	t.Helper()
	raw, err := os.ReadFile("testdata/native-v2-signing-vectors.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f conformanceFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return append(append([]conformanceVector{}, f.BaseVectors...), f.SupplementalVectors...)
}

func (v conformanceVector) descriptor(t *testing.T) []interface{} {
	t.Helper()
	switch v.AuthorizationKind {
	case 0:
		return singleDescriptor()
	case 1:
		if v.MultisigAccount == nil {
			t.Fatalf("%s: multisig kind without account", v.Name)
		}
		return multiDescriptor(common.HexToAddress(*v.MultisigAccount))
	default:
		t.Fatalf("%s: unknown authorization_kind %d", v.Name, v.AuthorizationKind)
		return nil
	}
}

func (v conformanceVector) proof(t *testing.T) interface{} {
	t.Helper()
	if v.AuthorizationKind == 0 {
		var p struct {
			Signature conformanceSig `json:"signature"`
		}
		if err := json.Unmarshal(v.AuthorizationProof, &p); err != nil {
			t.Fatalf("%s: proof: %v", v.Name, err)
		}
		return singleProof(Signature{R: p.Signature.R, S: p.Signature.S, V: p.Signature.V})
	}
	var p struct {
		Signatures []struct {
			SignerPubkey string         `json:"signer_pubkey"`
			Signature    conformanceSig `json:"signature"`
		} `json:"signatures"`
	}
	if err := json.Unmarshal(v.AuthorizationProof, &p); err != nil {
		t.Fatalf("%s: proof: %v", v.Name, err)
	}
	entries := make([]multiSigProofEntry, 0, len(p.Signatures))
	for _, e := range p.Signatures {
		entries = append(entries, multiSigProofEntry{
			pubkey: common.FromHex(e.SignerPubkey),
			sig:    Signature{R: e.Signature.R, S: e.Signature.S, V: e.Signature.V},
		})
	}
	return multiProof(entries)
}

func TestNativeV2Conformance(t *testing.T) {
	vectors := loadVectors(t)
	if len(vectors) == 0 {
		t.Fatal("no vectors loaded")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			op := nativeOperationType(v.OperationType)
			descriptor := v.descriptor(t)
			payloadRLP := common.FromHex(v.PayloadRLP)

			unsigned, err := unsignedTxRLP(op, descriptor, payloadRLP)
			if err != nil {
				t.Fatalf("unsigned: %v", err)
			}
			if got := hexLower(unsigned); got != v.UnsignedTransactionRLP {
				t.Fatalf("unsigned_transaction_rlp\n got %s\nwant %s", got, v.UnsignedTransactionRLP)
			}
			if got := hexLower(crypto.Keccak256(unsigned)); got != v.SigningHash {
				t.Fatalf("signing_hash\n got %s\nwant %s", got, v.SigningHash)
			}

			proof := v.proof(t)
			signed, err := signedTxRLP(op, descriptor, payloadRLP, proof)
			if err != nil {
				t.Fatalf("signed: %v", err)
			}
			if got := hexLower(signed); got != v.SignedTransactionRLP {
				t.Fatalf("signed_transaction_rlp\n got %s\nwant %s", got, v.SignedTransactionRLP)
			}
			if got := hexLower(crypto.Keccak256(signed)); got != v.TransactionHash {
				t.Fatalf("transaction_hash\n got %s\nwant %s", got, v.TransactionHash)
			}
		})
	}
}
