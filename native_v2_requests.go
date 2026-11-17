package onemoney

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// txHasher is implemented by every submit response so the v2 path can verify
// the server-returned transaction hash against the locally computed one.
type txHasher interface {
	TxHash() string
}

// nativeAuthorization is the tagged /v2 authorization union. The current API
// produces the single-signature form only.
type nativeAuthorization struct {
	Type      string     `json:"type"`
	Signature *Signature `json:"signature,omitempty"`
}

func singleAuthorization(sig Signature) nativeAuthorization {
	s := sig
	return nativeAuthorization{Type: "single_secp256k1", Signature: &s}
}

// nativeV2Op carries everything the submit core needs for one operation.
type nativeV2Op struct {
	op          nativeOperationType
	payloadList []interface{}          // canonical RLP field list (native_v2_encoding.go)
	fields      map[string]interface{} // flattened JSON body fields (native_v2_encoding.go)
	pathV1      string
	pathV2      string
}

// payloadRLP builds payload_rlp for the canonical native-v2 form. Every canonical
// operation is WithMemo<Payload>; there is no bare-payload alternative.
func (op nativeV2Op) payloadRLP(memo Memo) ([]byte, error) {
	return encodeWithMemo(op.payloadList, memo)
}

// TransactionResponse is the minimal submission response (just the transaction
// hash), returned by the generic Client.Submit for offline / externally-signed
// flows. The namespace methods return richer operation-specific response types.
type TransactionResponse struct {
	Hash string `json:"hash"`
}

// TxHash reports the submitted transaction hash.
func (r *TransactionResponse) TxHash() string { return r.Hash }

// submitAuthorized POSTs an authorized transaction and verifies the
// server-returned hash against the locally computed one (fail-closed). It is the
// single submission implementation shared by the namespace API and the public
// Submit.
func (c *Client) submitAuthorized(ctx context.Context, authorized *AuthorizedTransaction, out txHasher) error {
	if authorized == nil {
		return fmt.Errorf("nil authorized transaction")
	}
	if err := c.PostMethod(ctx, authorized.path, authorized.body, out); err != nil {
		return err
	}
	local := hexLower(authorized.txHash)
	if !strings.EqualFold(out.TxHash(), local) {
		return fmt.Errorf("transaction hash mismatch: server returned %s, locally computed %s", out.TxHash(), local)
	}
	return nil
}

// Submit sends an AuthorizedTransaction — typically produced offline via
// PrepareTransaction + Authorize — and verifies the returned hash. It returns
// the transaction hash; operation-specific response fields (e.g. an issued
// token address) are available through the namespace methods instead.
func (c *Client) Submit(ctx context.Context, authorized *AuthorizedTransaction) (*TransactionResponse, error) {
	out := new(TransactionResponse)
	if err := c.submitAuthorized(ctx, authorized, out); err != nil {
		return nil, err
	}
	return out, nil
}

// buildLegacyV1Body signs the bare payload and assembles the legacy v1 request
// body (top-level signature, no memo/authorization). No network I/O.
func buildLegacyV1Body(op nativeV2Op, signer Signer) (map[string]interface{}, error) {
	bareRLP, err := rlp.EncodeToBytes(op.payloadList)
	if err != nil {
		return nil, err
	}
	sig, err := signer.SignHash(crypto.Keccak256(bareRLP))
	if err != nil {
		return nil, err
	}
	body := op.fields
	body["signature"] = sig
	return body, nil
}

func (c *Client) submitLegacyV1(ctx context.Context, op nativeV2Op, signer Signer, out txHasher) error {
	body, err := buildLegacyV1Body(op, signer)
	if err != nil {
		return err
	}
	return c.PostMethod(ctx, op.pathV1, body, out)
}

// TxHash methods let each response satisfy txHasher for v2 hash verification.
func (r *IssueTokenResponse) TxHash() string         { return r.Hash }
func (r *UpdateMetadataResponse) TxHash() string     { return r.Hash }
func (r *GrantAuthorityResponse) TxHash() string     { return r.Hash }
func (r *MintTokenResponse) TxHash() string          { return r.Hash }
func (r *BridgeAndMintTokenResponse) TxHash() string { return r.Hash }
func (r *BurnTokenResponse) TxHash() string          { return r.Hash }
func (r *BurnAndBridgeTokenResponse) TxHash() string { return r.Hash }
func (r *SetTokenManageListResponse) TxHash() string { return r.Hash }
func (r *PauseTokenResponse) TxHash() string         { return r.Hash }

// pathsForOp returns the legacy v1 and domain-separated v2 REST paths for an
// operation. Single source of truth for native write endpoints.
func pathsForOp(op nativeOperationType) (v1, v2 string, err error) {
	switch op {
	case opPayment:
		return "/v1/transactions/payment", "/v2/transactions/payment", nil
	case opBatchPayment:
		return "", "/v2/transactions/batch_payment", nil // v2-only; the /v1 route is deprecated on L1
	case opTokenIssue:
		return "/v1/tokens/issue", "/v2/tokens/issue", nil
	case opTokenMint:
		return "/v1/tokens/mint", "/v2/tokens/mint", nil
	case opTokenBurn:
		return "/v1/tokens/burn", "/v2/tokens/burn", nil
	case opTokenBridgeAndMint:
		return "/v1/tokens/bridge_and_mint", "/v2/tokens/bridge_and_mint", nil
	case opTokenBurnAndBridge:
		return "/v1/tokens/burn_and_bridge", "/v2/tokens/burn_and_bridge", nil
	case opTokenAuthority:
		return "/v1/tokens/grant_authority", "/v2/tokens/grant_authority", nil
	case opTokenClawback:
		return "/v1/tokens/clawback", "/v2/tokens/clawback", nil
	case opTokenBlacklist:
		return "/v1/tokens/manage_blacklist", "/v2/tokens/manage_blacklist", nil
	case opTokenWhitelist:
		return "/v1/tokens/manage_whitelist", "/v2/tokens/manage_whitelist", nil
	case opTokenPause:
		return "/v1/tokens/pause", "/v2/tokens/pause", nil
	case opTokenMetadata:
		return "/v1/tokens/update_metadata", "/v2/tokens/update_metadata", nil
	case opCreateMultiSig:
		return "", "/v2/accounts/multisig", nil // v2-only, no legacy form
	case opSettledAuthority:
		return "", "/v2/tokens/authority_operation", nil
	case opLaneClawback:
		return "", "/v2/tokens/lane_clawback", nil
	default:
		return "", "", fmt.Errorf("%s: no domain-separated v2 endpoint configured", op.label())
	}
}

// opFromPayload builds a complete submit operation from a payload value and
// config, sourcing operation type, RLP list, body fields, and REST paths from
// the single central mappings.
func opFromPayload(payload any, cfg submitConfig) (nativeV2Op, error) {
	op, list, fields, err := resolvePayloadOp(payload, cfg)
	if err != nil {
		return nativeV2Op{}, err
	}
	v1, v2, err := pathsForOp(op)
	if err != nil {
		return nativeV2Op{}, err
	}
	return nativeV2Op{
		op:          op,
		payloadList: list,
		fields:      fields,
		pathV1:      v1,
		pathV2:      v2,
	}, nil
}

// submitPayload runs a payload through the submission pipeline per the client's
// mode. For domain-separated v2 it uses the exact pipeline callers use offline —
// PrepareTransaction -> Authorize -> Submit — with signing and submission
// automated via the Signer, so the one-step API and the offline API share a
// single implementation. out must be a pointer response implementing txHasher.
func (c *Client) submitPayload(ctx context.Context, payload any, cfg submitConfig, signer Signer, out txHasher) error {
	if signer == nil {
		return fmt.Errorf("nil signer: pass a Signer (e.g. NewPrivateKeySigner or a KMS/HSM-backed one)")
	}
	// Legacy v1 signs the bare payload and does not use a PreparedTransaction.
	// Resolution runs first so a v2-only operation reports its own capability
	// error rather than being masked by the generic legacy-memo guard.
	if c.mode() == SubmissionModeLegacyV1 {
		op, err := opFromPayload(payload, cfg)
		if err != nil {
			return err
		}
		if op.pathV1 == "" {
			return fmt.Errorf("%s requires domain-separated v2 submission mode", op.op.label())
		}
		if cfg.memoSet {
			return fmt.Errorf("memo is not supported in legacy v1 submission mode; use the default v2 mode to sign a memo")
		}
		if cfg.onSigned != nil {
			return fmt.Errorf("WithOnSigned is not supported in legacy v1 submission mode: a /v1 request has no AuthorizedTransaction to hand over")
		}
		return c.submitLegacyV1(ctx, op, signer, out)
	}
	_, err := c.signAndSubmitV2(ctx, payload, cfg, signer, out)
	return err
}

// signAndSubmitV2 signs payload as a domain-separated v2 transaction, hands it
// to cfg.onSigned, and sends it only once that returns nil. It returns the
// signed transaction whenever one was signed, also with a send error.
func (c *Client) signAndSubmitV2(ctx context.Context, payload any, cfg submitConfig, signer Signer, out txHasher) (*AuthorizedTransaction, error) {
	// Every canonical v2 operation is memo-bearing, so a memo needs no capability
	// check here — prepareFromPayload folds it into the signed preimage.
	prepared, err := prepareFromPayload(payload, cfg)
	if err != nil {
		return nil, err
	}
	if _, storable := storableBodies[prepared.pathV2]; cfg.onSigned != nil && !storable {
		return nil, fmt.Errorf("WithOnSigned supports only authority-class transactions, which have a stored form; %s has none", prepared.op.label())
	}
	sig, err := signer.SignHash(prepared.SigningHash())
	if err != nil {
		return nil, err
	}
	if cfg.verifySigner {
		if err := verifySignerAddress(prepared.SigningHash(), sig, signer.Address()); err != nil {
			return nil, err
		}
	}
	authorized, err := prepared.Authorize(sig)
	if err != nil {
		return nil, err
	}
	if cfg.onSigned != nil {
		if err := cfg.onSigned(ctx, SignedTransaction{Transaction: authorized, Replaces: cfg.replaces}); err != nil {
			hash := common.BytesToHash(authorized.txHash)
			return nil, fmt.Errorf("the store of signed transaction %s failed, so it was not sent: %w", hash.Hex(), err)
		}
	}
	return authorized, c.submitAuthorized(ctx, authorized, out)
}

// verifySignerAddress checks that the signature recovers to the address that
// the Signer reports. A clawback guards a second debit with that address's
// nonce, so a Signer whose Address belongs to another key must fail before
// anything is stored or sent.
func verifySignerAddress(signingHash []byte, sig Signature, want common.Address) error {
	r, s, v, err := sigComponents(sig)
	if err != nil {
		return err
	}
	if v > 1 {
		return fmt.Errorf("invalid signature v: %d; must be 0 or 1", v)
	}
	raw := make([]byte, 65)
	r.FillBytes(raw[:32])
	s.FillBytes(raw[32:64])
	raw[64] = byte(v)
	pub, err := crypto.SigToPub(signingHash, raw)
	if err != nil {
		return fmt.Errorf("recover the signer: %w", err)
	}
	if got := crypto.PubkeyToAddress(*pub); got != want {
		return fmt.Errorf("the Signer reports address %s but signs with the key of %s: nothing was stored or sent", want.Hex(), got.Hex())
	}
	return nil
}
