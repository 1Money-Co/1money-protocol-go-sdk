package onemoney

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func authorizeForAlias(t *testing.T, payload any) *AuthorizedTransaction {
	t.Helper()
	prep, err := PrepareTransaction(payload)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := prep.Authorize(sig)
	if err != nil {
		t.Fatal(err)
	}
	return authorized
}

// TestAuthorizedBodyDoesNotAliasPayload asserts that mutating the original
// payload after Authorize does not change the (documented-immutable) request
// body — the reference-typed fields must be deep-copied when the body is built,
// or the submitted JSON would diverge from the signed hashes and the node would
// reject the transaction.
func TestAuthorizedBodyDoesNotAliasPayload(t *testing.T) {
	t.Run("additional_metadata", func(t *testing.T) {
		payload := UpdateMetadataPayload{
			ChainID: 1, Nonce: 1, Name: "n", URI: "u", Token: repeatAddr(0x01),
			AdditionalMetadata: []AdditionalMetadata{{Key: "k", Value: "v"}},
		}
		authorized := authorizeForAlias(t, payload)
		before, _ := json.Marshal(authorized.body)
		payload.AdditionalMetadata[0].Value = "MUTATED"
		after, _ := json.Marshal(authorized.body)
		if !bytes.Equal(before, after) {
			t.Errorf("body aliases additional_metadata:\n before=%s\n after =%s", before, after)
		}
	})

	t.Run("bridge_param", func(t *testing.T) {
		payload := TokenBurnAndBridgePayload{
			ChainID: 1, Nonce: 1, Sender: repeatAddr(0x0b), Value: big.NewInt(1), Token: repeatAddr(0x01),
			DestinationChainID: 1, DestinationAddress: "0xdest", EscrowFee: big.NewInt(0),
			BridgeMetadata: "", BridgeParam: HexBytes{0x01, 0x02, 0x03},
		}
		authorized := authorizeForAlias(t, payload)
		before, _ := json.Marshal(authorized.body)
		payload.BridgeParam[0] = 0xFF
		after, _ := json.Marshal(authorized.body)
		if !bytes.Equal(before, after) {
			t.Errorf("body aliases bridge_param:\n before=%s\n after =%s", before, after)
		}
	})

	t.Run("operations_hash", func(t *testing.T) {
		h := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
		payload := BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
			Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1)}},
			MaxFee:     big.NewInt(1), CreatedAt: 1, OperationsHash: &h,
		}
		authorized := authorizeForAlias(t, payload)
		before, _ := json.Marshal(authorized.body)
		h[0] = 0xFF // mutate the pointed-to hash
		after, _ := json.Marshal(authorized.body)
		if !bytes.Equal(before, after) {
			t.Errorf("body aliases operations_hash:\n before=%s\n after =%s", before, after)
		}
	})
}
