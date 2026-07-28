package onemoney

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// nativeTxDomainV2 is the single frozen domain for the domain-separated v2
// canonical native transaction (issue #1038 / native-v2-signing-spec §2.1).
const nativeTxDomainV2 = "1money.native.transaction.v2"

// nativeOperationType is the frozen u16 operation-type registry
// (native-v2-signing-spec §2.2). Values are never reordered or reused.
type nativeOperationType uint16

const (
	opPayment            nativeOperationType = 1
	opTokenIssue         nativeOperationType = 2
	opTokenMint          nativeOperationType = 3
	opTokenAuthority     nativeOperationType = 4
	opTokenBlacklist     nativeOperationType = 5
	opTokenWhitelist     nativeOperationType = 6
	opTokenPause         nativeOperationType = 7
	opTokenBurn          nativeOperationType = 8
	opTokenClawback      nativeOperationType = 9
	opTokenMetadata      nativeOperationType = 10
	opTokenBridgeAndMint nativeOperationType = 11
	opTokenBurnAndBridge nativeOperationType = 12
	opCreateMultiSig     nativeOperationType = 13
	opBatchPayment       nativeOperationType = 14
)

// memoList encodes a Memo as the always-present 3-element RLP list of UTF-8
// strings (native-v2-signing-spec §4.1).
func memoList(m Memo) []interface{} {
	return []interface{}{[]byte(m.Type), []byte(m.Format), []byte(m.Data)}
}

// encodeWithMemo builds payload_rlp = rlp([ payloadList, memoList ]) for a
// memo-capable operation.
func encodeWithMemo(payloadList []interface{}, m Memo) ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{payloadList, memoList(m)})
}

// encodeBare builds payload_rlp = rlp(payloadList) for an operation with no
// memo wrapper (BatchPayment).
func encodeBare(payloadList []interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(payloadList)
}

// singleDescriptor is the SingleSecp256k1 authorization descriptor: [0].
func singleDescriptor() []interface{} { return []interface{}{uint64(0)} }

// multiDescriptor is the MultiSecp256k1 authorization descriptor: [1, account].
func multiDescriptor(account common.Address) []interface{} {
	return []interface{}{uint64(1), account}
}

// unsignedTxRLP = rlp([ DOMAIN, opType, descriptor, payloadRLP ]). payloadRLP is
// embedded as one opaque byte-string element (native-v2-signing-spec §5).
func unsignedTxRLP(op nativeOperationType, descriptor []interface{}, payloadRLP []byte) ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		[]byte(nativeTxDomainV2), uint64(op), descriptor, payloadRLP,
	})
}

// signingHashV2 = keccak256(unsignedTxRLP).
func signingHashV2(op nativeOperationType, descriptor []interface{}, payloadRLP []byte) ([]byte, error) {
	unsigned, err := unsignedTxRLP(op, descriptor, payloadRLP)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(unsigned), nil
}

// signedTxRLP = rlp([ DOMAIN, opType, descriptor, payloadRLP, proof ]) — the
// proof appended as a fifth element and the whole list re-encoded
// (native-v2-signing-spec §7).
func signedTxRLP(op nativeOperationType, descriptor []interface{}, payloadRLP []byte, proof interface{}) ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		[]byte(nativeTxDomainV2), uint64(op), descriptor, payloadRLP, proof,
	})
}

// txHashV2 = keccak256(signedTxRLP).
func txHashV2(op nativeOperationType, descriptor []interface{}, payloadRLP []byte, proof interface{}) ([]byte, error) {
	signed, err := signedTxRLP(op, descriptor, payloadRLP, proof)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(signed), nil
}

// sigComponents converts a Signature's hex r/s and v into RLP-ready values.
func sigComponents(sig Signature) (*big.Int, *big.Int, uint64) {
	r := new(big.Int).SetBytes(common.FromHex(sig.R))
	s := new(big.Int).SetBytes(common.FromHex(sig.S))
	return r, s, sig.V
}

// singleProof builds authorization_proof_rlp(single) = [r, s, v].
func singleProof(sig Signature) []interface{} {
	r, s, v := sigComponents(sig)
	return []interface{}{r, s, v}
}

// multiSigProofEntry is one signer's contribution to a multisig proof.
type multiSigProofEntry struct {
	pubkey []byte // 33-byte SEC1-compressed, encoded as an ordinary string here
	sig    Signature
}

// multiProof builds authorization_proof_rlp(multi) = [[pubkey, r, s, v], ...].
// Entries must already be in strictly-ascending compressed-pubkey order.
func multiProof(entries []multiSigProofEntry) []interface{} {
	out := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		r, s, v := sigComponents(e.sig)
		out = append(out, []interface{}{e.pubkey, r, s, v})
	}
	return out
}

// secp256k1N is the secp256k1 group order and secp256k1HalfN = N/2 is the low-S
// ceiling. The L1 signer recovery rejects any signature with s > N/2
// (CryptoError::HighSSignature), so the SDK enforces the same bound before submit.
var (
	secp256k1N     = crypto.S256().Params().N
	secp256k1HalfN = new(big.Int).Rsh(crypto.S256().Params().N, 1)
)

// validateSignatureComponents enforces exactly what the node applies during
// signature recovery: v is the 0/1 y-parity, r and s are in [1, N), and s is
// canonical low-S (s <= N/2). Rejecting a high-S signature here makes a custom
// KMS/HSM signer fail fast with a clear message instead of a confusing
// server-side HighSSignature rejection.
func validateSignatureComponents(sig Signature) error {
	if sig.V > 1 {
		return fmt.Errorf("invalid signature v: %d; must be 0 or 1 (y-parity, not the legacy Ethereum 27/28)", sig.V)
	}
	r := new(big.Int).SetBytes(common.FromHex(sig.R))
	s := new(big.Int).SetBytes(common.FromHex(sig.S))
	if r.Sign() <= 0 || r.Cmp(secp256k1N) >= 0 {
		return fmt.Errorf("invalid signature r: out of range [1, N)")
	}
	if s.Sign() <= 0 || s.Cmp(secp256k1N) >= 0 {
		return fmt.Errorf("invalid signature s: out of range [1, N)")
	}
	if s.Cmp(secp256k1HalfN) > 0 {
		return fmt.Errorf("invalid signature s: not canonical low-S; the node rejects high-S (submit the normalized s = N - s)")
	}
	return nil
}

func bigOrZero(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return v
}

func boolToUint(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// hexLower renders bytes as a lowercase 0x-prefixed hex string.
func hexLower(b []byte) string { return "0x" + common.Bytes2Hex(b) }
