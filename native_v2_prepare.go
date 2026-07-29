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

func validatePayloadU256(payload any) error {
	switch p := payload.(type) {
	case PaymentPayload:
		return validateU256("payment.value", p.Value)
	case BatchPaymentPayload:
		if err := validateU256("batch.max_fee", p.MaxFee); err != nil {
			return err
		}
		for index, operation := range p.Operations {
			if err := validateU256(fmt.Sprintf("batch.operations[%d].amount", index), operation.Amount); err != nil {
				return err
			}
		}
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
//   - memoCapable: whether the operation carries a memo (false only for
//     BatchPayment).
//   - err:         non-nil for an unsupported payload type, or an ambiguous
//     TokenManageListPayload with no WithManageListKind.
func resolvePayloadOp(payload any, cfg submitConfig) (op nativeOperationType, payloadList []interface{}, bodyFields map[string]interface{}, memoCapable bool, err error) {
	if err := validatePayloadU256(payload); err != nil {
		return 0, nil, nil, false, err
	}
	switch p := payload.(type) {
	case PaymentPayload:
		return opPayment, p.rlpList(), p.wireFields(), true, nil
	case BatchPaymentPayload:
		return opBatchPayment, p.rlpList(), p.wireFields(), false, nil
	case TokenIssuePayload:
		return opTokenIssue, p.rlpList(), p.wireFields(), true, nil
	case TokenMintPayload:
		return opTokenMint, p.rlpList(), p.wireFields(), true, nil
	case TokenBurnPayload:
		return opTokenBurn, p.rlpList(), p.wireFields(), true, nil
	case TokenBridgeAndMintPayload:
		return opTokenBridgeAndMint, p.rlpList(), p.wireFields(), true, nil
	case TokenBurnAndBridgePayload:
		return opTokenBurnAndBridge, p.rlpList(), p.wireFields(), true, nil
	case TokenAuthorityPayload:
		return opTokenAuthority, p.rlpList(), p.wireFields(), true, nil
	case TokenClawbackPayload:
		return opTokenClawback, p.rlpList(), p.wireFields(), true, nil
	case PauseTokenPayload:
		return opTokenPause, p.rlpList(), p.wireFields(), true, nil
	case UpdateMetadataPayload:
		return opTokenMetadata, p.rlpList(), p.wireFields(), true, nil
	case CreateMultiSigPayload:
		if err := validateMultisigConfig(p.Signers, p.Threshold); err != nil {
			return 0, nil, nil, false, err
		}
		return opCreateMultiSig, p.rlpList(), p.wireFields(), true, nil
	case TokenManageListPayload:
		if cfg.listKind == nil {
			return 0, nil, nil, false, fmt.Errorf("TokenManageListPayload is ambiguous: pass WithManageListKind(ManageListBlacklist or ManageListWhitelist)")
		}
		// The operation type is part of the signing domain, so an unknown kind
		// must error rather than silently map to blacklist.
		switch *cfg.listKind {
		case ManageListBlacklist:
			return opTokenBlacklist, p.rlpList(), p.wireFields(), true, nil
		case ManageListWhitelist:
			return opTokenWhitelist, p.rlpList(), p.wireFields(), true, nil
		default:
			return 0, nil, nil, false, fmt.Errorf("invalid ManageListKind %d", *cfg.listKind)
		}
	default:
		return 0, nil, nil, false, fmt.Errorf("unsupported payload type %T", payload)
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
	memoCapable bool
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
// on exactly one pipeline.
func prepareFromPayload(payload any, cfg submitConfig) (*PreparedTransaction, error) {
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
		memoCapable: op.memoCapable,
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
	txHash, err := txHashV2(p.op, p.descriptor, p.payloadRLP, singleProof(sig))
	if err != nil {
		return nil, err
	}
	body := make(map[string]interface{}, len(p.fields)+2)
	for k, v := range p.fields {
		body[k] = v
	}
	if p.memoCapable {
		body["memo"] = p.memo
	}
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
