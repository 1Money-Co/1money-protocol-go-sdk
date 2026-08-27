package v2operations_test

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

type exampleEnvironment struct {
	ctx     context.Context
	client  *onemoney.Client
	signer  onemoney.Signer
	chainID uint64
	nonce   uint64
}

// newExampleEnvironment shows the common setup used by every operation
// example. It is not run by go test because the Example functions intentionally
// have no Output directive; users copying an example must supply these values.
func newExampleEnvironment() exampleEnvironment {
	ctx := context.Background()
	client := onemoney.NewClientWithCustomUrl(requiredEnvironment("API_URL"))
	signer, err := onemoney.NewPrivateKeySigner(requiredEnvironment("PRIVATE_KEY"))
	if err != nil {
		panic(fmt.Errorf("create signer: %w", err))
	}

	chain, err := client.GetChainId(ctx)
	if err != nil {
		panic(fmt.Errorf("get chain ID: %w", err))
	}
	nonce, err := client.GetAccountNonce(ctx, signer.Address())
	if err != nil {
		panic(fmt.Errorf("get account nonce: %w", err))
	}

	return exampleEnvironment{
		ctx:     ctx,
		client:  client,
		signer:  signer,
		chainID: chain.ChainId,
		nonce:   nonce.Nonce,
	}
}

func requiredEnvironment(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		panic(fmt.Sprintf("%s is required", name))
	}
	return value
}

func requiredAddress(name string) common.Address {
	value := requiredEnvironment(name)
	if !common.IsHexAddress(value) {
		panic(fmt.Sprintf("%s must be a hex address", name))
	}
	address := common.HexToAddress(value)
	if address == (common.Address{}) {
		panic(fmt.Sprintf("%s must not be the zero address", name))
	}
	return address
}

func exampleMemo() onemoney.Memo {
	return onemoney.Memo{
		Type:   "purpose/example",
		Format: "text/plain",
		Data:   "v2 operation example",
	}
}

func (e exampleEnvironment) paymentPayload() onemoney.PaymentPayload {
	return onemoney.PaymentPayload{
		ChainID:   e.chainID,
		Nonce:     e.nonce,
		Recipient: requiredAddress("RECIPIENT_ADDRESS"),
		Value:     big.NewInt(100),
		Token:     requiredAddress("TOKEN_ADDRESS"),
	}
}

func (e exampleEnvironment) batchPaymentPayload() onemoney.BatchPaymentPayload {
	recipient := requiredAddress("RECIPIENT_ADDRESS")
	account := requiredAddress("ACCOUNT_ADDRESS")
	return onemoney.BatchPaymentPayload{
		ChainID: e.chainID,
		Nonce:   e.nonce,
		Token:   requiredAddress("TOKEN_ADDRESS"),
		Operations: []onemoney.PaymentOperation{
			{Recipient: recipient, Amount: big.NewInt(100)},
			{Recipient: account, Amount: big.NewInt(200)},
		},
		CreatedAt: uint64(time.Now().Unix()),
	}
}

func (e exampleEnvironment) issuePayload() onemoney.TokenIssuePayload {
	return onemoney.TokenIssuePayload{
		ChainID:         e.chainID,
		Nonce:           e.nonce,
		Symbol:          "EXAMPLE",
		Name:            "Example Token",
		Decimals:        6,
		MasterAuthority: e.signer.Address(),
		IsPrivate:       false,
		ClawbackEnabled: true,
	}
}

func (e exampleEnvironment) mintPayload() onemoney.TokenMintPayload {
	return onemoney.TokenMintPayload{
		ChainID:   e.chainID,
		Nonce:     e.nonce,
		Recipient: requiredAddress("RECIPIENT_ADDRESS"),
		Value:     big.NewInt(100),
		Token:     requiredAddress("TOKEN_ADDRESS"),
	}
}

func (e exampleEnvironment) burnPayload() onemoney.TokenBurnPayload {
	return onemoney.TokenBurnPayload{
		ChainID: e.chainID,
		Nonce:   e.nonce,
		Value:   big.NewInt(100),
		Token:   requiredAddress("TOKEN_ADDRESS"),
	}
}

func (e exampleEnvironment) bridgeAndMintPayload() onemoney.TokenBridgeAndMintPayload {
	return onemoney.TokenBridgeAndMintPayload{
		ChainID:        e.chainID,
		Nonce:          e.nonce,
		Recipient:      requiredAddress("RECIPIENT_ADDRESS"),
		Value:          big.NewInt(100),
		Token:          requiredAddress("TOKEN_ADDRESS"),
		SourceChainID:  1,
		SourceTxHash:   "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		BridgeMetadata: "example bridge deposit",
	}
}

func (e exampleEnvironment) burnAndBridgePayload() onemoney.TokenBurnAndBridgePayload {
	recipient := requiredAddress("RECIPIENT_ADDRESS")
	return onemoney.TokenBurnAndBridgePayload{
		ChainID:            e.chainID,
		Nonce:              e.nonce,
		Sender:             e.signer.Address(),
		Value:              big.NewInt(100),
		Token:              requiredAddress("TOKEN_ADDRESS"),
		DestinationChainID: 1,
		DestinationAddress: recipient.Hex(),
		EscrowFee:          big.NewInt(1),
		BridgeMetadata:     "example bridge withdrawal",
		BridgeParam:        onemoney.HexBytes{0xde, 0xad, 0xbe, 0xef},
	}
}

func (e exampleEnvironment) authorityPayload() onemoney.TokenAuthorityPayload {
	return onemoney.TokenAuthorityPayload{
		ChainID:          e.chainID,
		Nonce:            e.nonce,
		AuthorityType:    onemoney.AuthorityTypeMintBurnTokens,
		AuthorityAddress: requiredAddress("ACCOUNT_ADDRESS"),
		Token:            requiredAddress("TOKEN_ADDRESS"),
		Value:            big.NewInt(1_000),
	}
}

func (e exampleEnvironment) clawbackPayload() onemoney.TokenClawbackPayload {
	return onemoney.TokenClawbackPayload{
		ChainID:   e.chainID,
		Nonce:     e.nonce,
		Token:     requiredAddress("TOKEN_ADDRESS"),
		From:      requiredAddress("ACCOUNT_ADDRESS"),
		Recipient: requiredAddress("RECIPIENT_ADDRESS"),
		Value:     big.NewInt(100),
	}
}

func (e exampleEnvironment) manageListPayload() onemoney.TokenManageListPayload {
	return onemoney.TokenManageListPayload{
		ChainID: e.chainID,
		Nonce:   e.nonce,
		Action:  onemoney.ManageListActionAdd,
		Address: requiredAddress("ACCOUNT_ADDRESS"),
		Token:   requiredAddress("TOKEN_ADDRESS"),
	}
}

func (e exampleEnvironment) pausePayload() onemoney.PauseTokenPayload {
	return onemoney.PauseTokenPayload{
		ChainID: e.chainID,
		Nonce:   e.nonce,
		Token:   requiredAddress("TOKEN_ADDRESS"),
	}
}

func (e exampleEnvironment) updateMetadataPayload() onemoney.UpdateMetadataPayload {
	return onemoney.UpdateMetadataPayload{
		ChainID: e.chainID,
		Nonce:   e.nonce,
		Name:    "Updated Example Token",
		URI:     "https://example.com/token-metadata.json",
		Token:   requiredAddress("TOKEN_ADDRESS"),
		AdditionalMetadata: []onemoney.AdditionalMetadata{
			{Key: "category", Value: "example"},
		},
	}
}

func (e exampleEnvironment) createMultisigPayload() onemoney.CreateMultiSigPayload {
	return onemoney.CreateMultiSigPayload{
		ChainID: e.chainID,
		Nonce:   e.nonce,
		Signers: []onemoney.MultiSigSigner{
			{PublicKey: onemoney.HexBytes(e.signer.CompressedPublicKey()), Weight: 1},
		},
		Threshold: 1,
	}
}
