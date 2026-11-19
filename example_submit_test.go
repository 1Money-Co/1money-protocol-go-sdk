package onemoney_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"

	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

// A node that already holds the transaction answers in_progress. The SDK reads
// it as locked: resend the same signed transaction, never sign a new one.
func ExampleSubmitError() {
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"type":"https://developer.1moneynetwork.com/problem/submission_already_in_flight",`+
			`"action":"in_progress","message":"already inflight"}`)
	}))
	defer node.Close()

	signer, err := onemoney.NewPrivateKeySigner("0x01833a126ec45d0191519748146b9e35647aab7fed28de1c8e17824970f964a3")
	if err != nil {
		panic(err)
	}
	prepared, err := onemoney.PrepareTransaction(onemoney.PaymentPayload{
		ChainID: 1212101, Nonce: 0, Value: big.NewInt(10),
		Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
		Token:     common.HexToAddress("0x5458747a0efb9ebeb8696fcac1479278c0872fbe"),
	})
	if err != nil {
		panic(err)
	}
	signature, err := signer.SignHash(prepared.SigningHash())
	if err != nil {
		panic(err)
	}
	authorized, err := prepared.Authorize(signature)
	if err != nil {
		panic(err)
	}

	_, err = onemoney.NewClientWithCustomUrl(node.URL).Submit(context.Background(), authorized)
	var submitErr *onemoney.SubmitError
	if errors.As(err, &submitErr) {
		switch {
		case submitErr.RequestHadNoEffect():
			fmt.Println("rejected: this request had no effect")
		case submitErr.Kind == onemoney.SubmitHashMismatch:
			fmt.Println("the node admitted other bytes: send nothing more")
		default:
			// Locked or unknown: Client.Settle resends these same bytes.
			fmt.Println(submitErr.Kind, submitErr.Transaction == authorized)
		}
	}
	// Output:
	// locked true
}
