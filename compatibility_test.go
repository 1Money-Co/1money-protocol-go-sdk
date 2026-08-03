package onemoney

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"os/exec"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

type legacyHashableRequest interface {
	Hash() (common.Hash, error)
}

var (
	_ legacyHashableRequest = PaymentRequest{}
	_ legacyHashableRequest = IssueTokenRequest{}
	_ legacyHashableRequest = UpdateMetadataRequest{}
	_ legacyHashableRequest = TokenAuthorityRequest{}
	_ legacyHashableRequest = MintTokenRequest{}
	_ legacyHashableRequest = BridgeAndMintTokenRequest{}
	_ legacyHashableRequest = BurnTokenRequest{}
	_ legacyHashableRequest = BurnAndBridgeTokenRequest{}
	_ legacyHashableRequest = SetTokenManageListRequest{}
	_ legacyHashableRequest = PauseTokenRequest{}
)

func TestPrivateKeyToAddressCompatibility(t *testing.T) {
	const (
		privateKey = "0x0000000000000000000000000000000000000000000000000000000000000001"
		want       = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
	)

	got, err := PrivateKeyToAddress(privateKey)
	if err != nil {
		t.Fatalf("PrivateKeyToAddress() error = %v", err)
	}
	if got != want {
		t.Fatalf("PrivateKeyToAddress() = %q, want %q", got, want)
	}
}

func Test_PaymentRequest_Hash(t *testing.T) {
	value := big.NewInt(0)
	value.SetString("10", 10)

	paymentTx := PaymentRequest{
		PaymentPayload: PaymentPayload{
			ChainID:   1212101,
			Nonce:     0,
			Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
			Value:     value,
			Token:     common.HexToAddress("0x5458747a0efb9ebeb8696fcac1479278c0872fbe"),
		},
		Signature: Signature{
			R: "29799431026396113297345952769532737771367335026226509821050116192126323991602",
			S: "15357736211266391569611566560819218221258872050529851723622905759192743831009",
			V: 0,
		},
	}

	txHash, err := paymentTx.Hash()
	assert.Nil(t, err)
	assert.Equal(t, common.HexToHash("0xd002ef79e1b20b132d3bc679df4db240c891d5408c50b883f9020e9d65ac3740"), txHash)
}

func Test_PaymentRequest_Hash_withHexSignature(t *testing.T) {
	value := big.NewInt(0)
	value.SetString("10", 10)

	paymentTx := PaymentRequest{
		PaymentPayload: PaymentPayload{
			ChainID:   1212101,
			Nonce:     0,
			Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
			Value:     value,
			Token:     common.HexToAddress("0x5458747a0efb9ebeb8696fcac1479278c0872fbe"),
		},
		Signature: Signature{
			R: "0x41e1e158803da19ef1fc9ab35d86776cb02ac493265b948ff18b2c57a4e52432",
			S: "0x21f42bb02796a424b0961af374a71e0b948e8fadb58f1e5c6ac861be656265e1",
			V: 0,
		},
	}

	txHash, err := paymentTx.Hash()
	assert.Nil(t, err)
	assert.Equal(t, common.HexToHash("0xd002ef79e1b20b132d3bc679df4db240c891d5408c50b883f9020e9d65ac3740"), txHash)
}

func TestPublicAPICompatibility(t *testing.T) {
	cmd := exec.Command("go", "doc", "-all", ".")
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go doc -all .: %v\n%s", err, got)
	}

	index := -1
	for _, section := range []string{"CONSTANTS\n", "VARIABLES\n", "FUNCTIONS\n", "TYPES\n"} {
		if candidate := bytes.Index(got, []byte(section)); candidate >= 0 && (index < 0 || candidate < index) {
			index = candidate
		}
	}
	if index < 0 {
		t.Fatal("go doc output does not contain an exported API section")
	}
	got = got[index:]

	// Baseline of the exported API surface (from its first exported API section
	// onward; the package overview is excluded because doc.go intentionally adds
	// it). Update this hash deliberately whenever the public API changes on
	// purpose — e.g. the query-response types were aligned with the l1client REST
	// wire format (nullable checkpoint fields, batch/success receipt detail,
	// polymorphic signatures, BLS counter-signature, clawback metadata, typed
	// pricing), Transaction gained a MarshalJSON method so a decoded transaction
	// re-serializes with its data and authorization intact, and WithDebug was
	// added for verbose request/response logging. It guards against unintentional
	// drift, not intentional changes.
	const publicAPIHash = "f14701deb000cc000d493aaab815aeb4e65ed353091ec786cfbea7dbb1fe9bcb"
	if gotHash := fmt.Sprintf("%x", sha256.Sum256(got)); gotHash != publicAPIHash {
		t.Fatalf("public API hash = %s, want %s; compare `go doc -all .` with the baseline", gotHash, publicAPIHash)
	}
}
