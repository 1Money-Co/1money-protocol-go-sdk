package onemoney_test

import (
	"fmt"

	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
)

// Decide on the action, never on the message. On a submission, an empty or
// unknown action means check_status.
func ExampleParseProblem() {
	body := []byte(`{"type":"https://developer.1moneynetwork.com/problem/submission_already_in_flight",` +
		`"action":"in_progress","tx_hash":"0xabc","message":"already inflight"}`)
	p := onemoney.ParseProblem(body)

	switch p.Action {
	case onemoney.ActionInProgress:
		hash, _ := p.TxHash()
		fmt.Println("locked: resend the same signed transaction", hash)
	case onemoney.ActionFixRequest, onemoney.ActionRetryLater, onemoney.ActionUseOtherNode:
		fmt.Println("rejected before admission")
	default:
		fmt.Println("unknown: look up the transaction before you sign anything new")
	}
	// Output: locked: resend the same signed transaction 0xabc
}
