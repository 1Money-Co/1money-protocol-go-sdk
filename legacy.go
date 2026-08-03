package onemoney

// This file is the legacy-v1 compatibility boundary. Everything here exists only
// to keep pre-v2 code compiling and is deprecated in favor of the Signer-based
// Transactions()/Tokens()/Accounts() submit methods (domain-separated v2). New
// code should not use anything in this file. Deprecated domain submit methods
// (SendPayment, IssueToken, MintToken, ...) intentionally stay in their domain
// files beside their namespace replacements; this file collects the legacy
// request types and the exposed signing/hash helpers.

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// PrivateKeyToAddress returns the account address for a hex-encoded secp256k1
// private key (with or without a 0x prefix), as a 0x-hex string.
//
// Deprecated: use NewPrivateKeySigner(hex).Address(), which returns a
// common.Address directly. Kept for backward compatibility.
func PrivateKeyToAddress(privateKeyHex string) (string, error) {
	signer, err := NewPrivateKeySigner(privateKeyHex)
	if err != nil {
		return "", err
	}
	return signer.Address().Hex(), nil
}

// EncodePayload RLP-encodes a payload.
//
// Deprecated: v2 signing (domain separation) is handled internally by the
// Transactions()/Tokens()/Accounts() submit methods; this low-level legacy
// helper remains only for backward compatibility.
func EncodePayload(payload interface{}) ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return nil, fmt.Errorf("rlp encode payload failed: %w", err)
	}
	return encoded, nil
}

// HashMessage encodes the message via RLP and returns the Keccak256 hash.
//
// Deprecated: this is the legacy v1 (non-domain-separated) hash. New code uses
// the Transactions()/Tokens()/Accounts() submit methods, which compute the
// domain-separated v2 hash internally.
func HashMessage(msg interface{}) ([]byte, error) {
	encoded, err := EncodePayload(msg)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(encoded), nil
}

// SignMessage signs a payload with the legacy v1 scheme.
//
// Deprecated: signing is now handled internally by the Transactions()/Tokens()/
// Accounts() submit methods with a Signer, which use domain-separated v2
// signing by default. This method exposes the legacy scheme and remains only
// for backward compatibility.
func (client *Client) SignMessage(msg interface{}, privateKey string) (*Signature, error) {
	privateKey = strings.TrimPrefix(privateKey, "0x")
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	hash, err := HashMessage(msg)
	if err != nil {
		return nil, err
	}
	signature, err := crypto.Sign(hash, key)
	if err != nil {
		return nil, fmt.Errorf("sign message: %w", err)
	}
	return &Signature{
		R: common.BytesToHash(signature[:32]).Hex(),
		S: common.BytesToHash(signature[32:64]).Hex(),
		V: uint64(signature[64]),
	}, nil
}

// Hash computes the legacy v1 transaction hash for a signed payload.
//
// Deprecated: this is the pre-#1038 (non-domain-separated) hash and does not
// match domain-separated v2 transactions. For v2, use the hash returned by the
// submit response, or build it via PrepareTransaction(payload) →
// Authorize(sig) → AuthorizedTransaction.TransactionHash().
func Hash(payload interface{}, signature Signature) (common.Hash, error) {
	pEncode, err := EncodePayload(payload)
	if err != nil {
		return common.Hash{}, err
	}

	vEnc, err := encodeSignatureV(signature.V)
	if err != nil {
		return common.Hash{}, fmt.Errorf("%w", err)
	}

	rEnc, err := encodeSignatureRS(signature.R)
	if err != nil {
		return common.Hash{}, err
	}

	sEnc, err := encodeSignatureRS(signature.S)
	if err != nil {
		return common.Hash{}, err
	}

	vrsBytes := append(append(vEnc, rEnc...), sEnc...)

	totalLen := len(pEncode) + len(vrsBytes)
	header := encodeRLPListHeader(totalLen)

	txEnc := append(append(header, pEncode...), vrsBytes...)
	hash := crypto.Keccak256(txEnc)

	return common.BytesToHash(hash), nil
}

// encodeRLPListHeader encodes an RLP list header for the given content length.
func encodeRLPListHeader(length int) []byte {
	if length < 56 {
		return []byte{byte(0xc0 + length)}
	}
	var lenBytes []byte
	temp := length
	for temp > 0 {
		lenBytes = append([]byte{byte(temp & 0xff)}, lenBytes...)
		temp >>= 8
	}
	return append([]byte{byte(0xf7 + len(lenBytes))}, lenBytes...)
}

func encodeSignatureV(value uint64) ([]byte, error) {
	var vBytes []byte
	if value == 0 {
		vBytes = []byte{}
	} else {
		vBytes = []byte{byte(value)}
	}
	return rlp.EncodeToBytes(vBytes)
}

func encodeSignatureRS(value string) ([]byte, error) {
	base := 10
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		value = value[2:]
		base = 16
	}
	component, ok := big.NewInt(0).SetString(value, base)
	if !ok {
		return nil, errors.New("invalid signature component: expected base-10 string or 0x-prefixed hex string")
	}
	return rlp.EncodeToBytes(component)
}

// -----------------------------------------------------------------------------
// Legacy request wrappers (business payload + top-level signature)
// -----------------------------------------------------------------------------

// Deprecated: legacy v1 request type. Use Transactions().Payment with a Signer,
// which signs internally and defaults to domain-separated v2.
type PaymentRequest struct {
	PaymentPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r PaymentRequest) Hash() (common.Hash, error) {
	return Hash(r.PaymentPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().Issue with a Signer, which
// signs internally and defaults to domain-separated v2.
type IssueTokenRequest struct {
	TokenIssuePayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r IssueTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenIssuePayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().UpdateMetadata with a Signer,
// which signs internally and defaults to domain-separated v2.
type UpdateMetadataRequest struct {
	UpdateMetadataPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r UpdateMetadataRequest) Hash() (common.Hash, error) {
	return Hash(r.UpdateMetadataPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().GrantAuthority or
// Tokens().RevokeAuthority with a Signer, which sign internally and default to
// domain-separated v2.
type TokenAuthorityRequest struct {
	TokenAuthorityPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r TokenAuthorityRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenAuthorityPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().Mint with a Signer, which
// signs internally and defaults to domain-separated v2.
type MintTokenRequest struct {
	TokenMintPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r MintTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenMintPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().BridgeAndMint with a Signer,
// which signs internally and defaults to domain-separated v2.
type BridgeAndMintTokenRequest struct {
	TokenBridgeAndMintPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r BridgeAndMintTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenBridgeAndMintPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().Burn with a Signer, which
// signs internally and defaults to domain-separated v2.
type BurnTokenRequest struct {
	TokenBurnPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r BurnTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenBurnPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().BurnAndBridge with a Signer,
// which signs internally and defaults to domain-separated v2.
type BurnAndBridgeTokenRequest struct {
	TokenBurnAndBridgePayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r BurnAndBridgeTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenBurnAndBridgePayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().ManageBlacklist or
// Tokens().ManageWhitelist with a Signer, which sign internally and default to
// domain-separated v2.
type SetTokenManageListRequest struct {
	TokenManageListPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r SetTokenManageListRequest) Hash() (common.Hash, error) {
	return Hash(r.TokenManageListPayload, r.Signature)
}

// Deprecated: legacy v1 request type. Use Tokens().Pause or Tokens().Unpause
// with a Signer, which sign internally and default to domain-separated v2.
type PauseTokenRequest struct {
	PauseTokenPayload
	Signature Signature `json:"signature"`
}

// Hash returns the legacy v1 transaction hash (payload + signature).
//
// Deprecated: legacy v1 signing hash. The v2 namespace methods (Tokens() /
// Transactions()) hash internally; PrepareTransaction exposes SigningHash for
// offline signing.
func (r PauseTokenRequest) Hash() (common.Hash, error) {
	return Hash(r.PauseTokenPayload, r.Signature)
}
