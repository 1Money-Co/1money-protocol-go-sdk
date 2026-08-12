package onemoney

import (
	"fmt"
	"math/big"
)

// ManageListKind selects blacklist vs whitelist for a TokenManageListPayload,
// which share one Go type but map to different native operations (and therefore
// different signing hashes). Required by PrepareTransaction for that payload.
type ManageListKind int

const (
	// ManageListBlacklist is the blacklist operation (native op type 5).
	ManageListBlacklist ManageListKind = iota
	// ManageListWhitelist is the whitelist operation (native op type 6).
	ManageListWhitelist
)

// WithManageListKind disambiguates a TokenManageListPayload as blacklist or
// whitelist when preparing it.
func WithManageListKind(k ManageListKind) SubmitOption {
	return func(c *submitConfig) { c.listKind = &k }
}

func validateU256(name string, value *big.Int) error {
	if value == nil {
		return nil
	}
	if value.Sign() < 0 {
		return fmt.Errorf("%s must be non-negative", name)
	}
	if value.BitLen() > 256 {
		return fmt.Errorf("%s exceeds U256", name)
	}
	return nil
}

// validatePayloadEncodable checks only that a payload's numeric fields can be
// canonically encoded: every non-nil U256 is non-negative and fits in 256 bits.
// It says nothing about whether the node would accept the transaction --
// see validatePayloadAdmissible for that. Encoding is well-defined for payloads
// the node rejects, and the golden-vector fixtures deliberately pin some of
// those encodings.
func validatePayloadEncodable(payload any) error {
	switch p := payload.(type) {
	case PaymentPayload:
		return validateU256("payment.value", p.Value)
	case BatchPaymentPayload:
		return validateBatchOperationAmounts(p.Operations)
	case TokenMintPayload:
		return validateU256("mint.value", p.Value)
	case TokenBurnPayload:
		return validateU256("burn.value", p.Value)
	case TokenBridgeAndMintPayload:
		return validateU256("bridge_and_mint.value", p.Value)
	case TokenBurnAndBridgePayload:
		if err := validateU256("burn_and_bridge.value", p.Value); err != nil {
			return err
		}
		return validateU256("burn_and_bridge.escrow_fee", p.EscrowFee)
	case TokenAuthorityPayload:
		return validateU256("authority.value", p.Value)
	case TokenClawbackPayload:
		return validateU256("clawback.value", p.Value)
	}
	return nil
}

// resolvePayloadOp maps a supported payload value to its native operation. It is
// the single source of truth for the payload -> operation mapping, used by both
// PrepareTransaction (offline hashing) and the submit path.
//
// Return values:
//   - op:          the frozen native operation type (native-v2-signing-spec §2.2).
//   - payloadList: the canonical RLP field list, in signing order — the input to
//     the domain-separated signing hash.
//   - bodyFields:  the flattened JSON request-body fields (e.g. U256 amounts as
//     decimal strings, addresses as 0x-hex); memo and authorization are added
//     later, at Authorize time.
//   - err:         non-nil for an unsupported payload type, or an ambiguous
//     TokenManageListPayload with no WithManageListKind.
func resolvePayloadOp(payload any, cfg submitConfig) (op nativeOperationType, payloadList []interface{}, bodyFields map[string]interface{}, err error) {
	if err := validatePayloadEncodable(payload); err != nil {
		return 0, nil, nil, err
	}
	switch p := payload.(type) {
	case PaymentPayload:
		return opPayment, p.rlpList(), p.wireFields(), nil
	case BatchPaymentPayload:
		return opBatchPayment, p.rlpList(), p.wireFields(), nil
	case TokenIssuePayload:
		return opTokenIssue, p.rlpList(), p.wireFields(), nil
	case TokenMintPayload:
		return opTokenMint, p.rlpList(), p.wireFields(), nil
	case TokenBurnPayload:
		return opTokenBurn, p.rlpList(), p.wireFields(), nil
	case TokenBridgeAndMintPayload:
		return opTokenBridgeAndMint, p.rlpList(), p.wireFields(), nil
	case TokenBurnAndBridgePayload:
		return opTokenBurnAndBridge, p.rlpList(), p.wireFields(), nil
	case TokenAuthorityPayload:
		return opTokenAuthority, p.rlpList(), p.wireFields(), nil
	case TokenClawbackPayload:
		return opTokenClawback, p.rlpList(), p.wireFields(), nil
	case PauseTokenPayload:
		return opTokenPause, p.rlpList(), p.wireFields(), nil
	case UpdateMetadataPayload:
		return opTokenMetadata, p.rlpList(), p.wireFields(), nil
	case CreateMultiSigPayload:
		if err := validateMultisigConfig(p.Signers, p.Threshold); err != nil {
			return 0, nil, nil, err
		}
		return opCreateMultiSig, p.rlpList(), p.wireFields(), nil
	case TokenManageListPayload:
		if cfg.listKind == nil {
			return 0, nil, nil, fmt.Errorf("TokenManageListPayload is ambiguous: pass WithManageListKind(ManageListBlacklist or ManageListWhitelist)")
		}
		// The operation type is part of the signing domain, so an unknown kind
		// must error rather than silently map to blacklist.
		switch *cfg.listKind {
		case ManageListBlacklist:
			return opTokenBlacklist, p.rlpList(), p.wireFields(), nil
		case ManageListWhitelist:
			return opTokenWhitelist, p.rlpList(), p.wireFields(), nil
		default:
			return 0, nil, nil, fmt.Errorf("invalid ManageListKind %d", *cfg.listKind)
		}
	default:
		return 0, nil, nil, fmt.Errorf("unsupported payload type %T", payload)
	}
}

// PreparedTransaction is a built, unsigned domain-separated v2 transaction. It
// exposes the exact digest a signer must sign (SigningHash) and, once a
// signature is available, produces a submittable AuthorizedTransaction
// (Authorize) — sharing the exact pipeline the one-step namespace API uses
// internally. Build one with PrepareTransaction. Preparation and hashing are
// offline (no network).
type PreparedTransaction struct {
	op          nativeOperationType
	descriptor  []interface{}
	payloadRLP  []byte
	signingHash []byte
	fields      map[string]interface{}
	memo        Memo
	pathV2      string
}

// PrepareTransaction builds an unsigned single-signature v2 transaction for a
// supported payload value (PaymentPayload, TokenMintPayload, …). Pass WithMemo
// to attach a memo; pass WithManageListKind for a TokenManageListPayload. It
// returns an error for an unsupported or ambiguous payload.
func PrepareTransaction(payload any, opts ...SubmitOption) (*PreparedTransaction, error) {
	return prepareFromPayload(payload, resolveSubmit(opts))
}

// prepareFromPayload resolves a payload to its operation and builds the
// PreparedTransaction. It is the single payload -> prepared path, shared by the
// public PrepareTransaction (offline) and the namespace submit path, so both run
// on exactly one pipeline. Every canonical native-v2 operation carries a memo,
// so there is no memo-capability guard here.
// The three gates run in the order the node reaches the same conclusions, so a
// caller that keys on the error to decide what to fix is pointed at the same
// field the node would point at:
//
//  1. Encoding validity and operation resolution. On the node this is request
//     deserialization, inside the JSON extractor — a value with no U256 wire form
//     (negative, wider than 256 bits) fails there, before the verifier runs at
//     all.
//  2. Memo. The verifier validates the memo for every origin
//     (om-verifier/src/transaction_verifier.rs) before any operation-specific
//     rule.
//  3. Operation-specific static admission rules, which the verifier reaches last.
//
// Swapping 1 and 2 would report a bad memo for a payload whose amount cannot be
// deserialized at all, which is not what the node does.
func prepareFromPayload(payload any, cfg submitConfig) (*PreparedTransaction, error) {
	op, err := opFromPayload(payload, cfg)
	if err != nil {
		return nil, err
	}
	if err := cfg.memo.validate(); err != nil {
		return nil, err
	}
	if err := validatePayloadAdmissible(payload); err != nil {
		return nil, err
	}
	return newPrepared(op, cfg.memo)
}

// validatePayloadAdmissible applies the node's static, governance-independent
// admission rules. It is the gate that separates "this encodes correctly" from
// "the node can accept this", and it runs before signing so a caller never
// spends a signing operation -- or an HSM round trip -- on a transaction that is
// certain to be rejected.
//
// Only BatchPayment carries such rules today. The node's remaining BatchPayment
// checks are governance-dependent (batch payments enabled, the
// operations-per-batch limit, the encoded-size limit, fee-asset matching) and
// stay with the server: the SDK would have to guess at governance state.
func validatePayloadAdmissible(payload any) error {
	if batch, ok := payload.(BatchPaymentPayload); ok {
		return validateBatchPaymentSubmission(batch)
	}
	return nil
}

// prepareCanonical builds a PreparedTransaction for a payload's canonical
// encoding, WITHOUT the admission gate.
//
// It exists because canonical encoding is well-defined for payloads the node
// would reject at admission -- an arbitrary operations_hash, an empty operation
// list, a zero amount -- and the golden-vector fixtures deliberately pin those
// encodings. Only fixture-conformance tests should use it. Every production
// entry point goes through prepareFromPayload, which gates first.
func prepareCanonical(payload any, cfg submitConfig) (*PreparedTransaction, error) {
	op, err := opFromPayload(payload, cfg)
	if err != nil {
		return nil, err
	}
	return newPrepared(op, cfg.memo)
}

// newPrepared builds a PreparedTransaction from an already-resolved operation.
// It is the op -> prepared step used by prepareFromPayload.
func newPrepared(op nativeV2Op, memo Memo) (*PreparedTransaction, error) {
	payloadRLP, err := op.payloadRLP(memo)
	if err != nil {
		return nil, err
	}
	descriptor := singleDescriptor()
	sh, err := signingHashV2(op.op, descriptor, payloadRLP)
	if err != nil {
		return nil, err
	}
	return &PreparedTransaction{
		op:          op.op,
		descriptor:  descriptor,
		payloadRLP:  payloadRLP,
		signingHash: sh,
		fields:      op.fields,
		memo:        memo,
		pathV2:      op.pathV2,
	}, nil
}

// SigningHash returns the 32-byte digest the signer must sign for this
// transaction. The returned slice is a copy and safe to retain.
func (p *PreparedTransaction) SigningHash() []byte {
	out := make([]byte, len(p.signingHash))
	copy(out, p.signingHash)
	return out
}

// Authorize attaches a single signature and returns a submittable
// AuthorizedTransaction with its final public transaction hash. The signature
// must be over SigningHash(). It is validated exactly as the node validates it:
// v must be the 0/1 y-parity (never the legacy 27/28), r and s must be in
// [1, N), and s must be canonical low-S (s <= N/2) — the node rejects high-S, so
// Authorize does too rather than producing a hash the server will not accept.
func (p *PreparedTransaction) Authorize(sig Signature) (*AuthorizedTransaction, error) {
	if err := validateSignatureComponents(sig); err != nil {
		return nil, err
	}
	proof, err := singleProof(sig)
	if err != nil {
		return nil, err
	}
	txHash, err := txHashV2(p.op, p.descriptor, p.payloadRLP, proof)
	if err != nil {
		return nil, err
	}
	body := make(map[string]interface{}, len(p.fields)+2)
	for k, v := range p.fields {
		body[k] = v
	}
	body["memo"] = p.memo
	body["authorization"] = singleAuthorization(sig)
	return &AuthorizedTransaction{op: p.op, txHash: txHash, body: body, path: p.pathV2}, nil
}

// AuthorizedTransaction is a signed, canonical v2 transaction ready for
// Client.Submit. Its fields are immutable after construction.
type AuthorizedTransaction struct {
	op     nativeOperationType
	txHash []byte
	body   map[string]interface{}
	path   string
}

// TransactionHash returns the public transaction hash of this authorized
// transaction (a copy).
func (a *AuthorizedTransaction) TransactionHash() []byte {
	out := make([]byte, len(a.txHash))
	copy(out, a.txHash)
	return out
}
