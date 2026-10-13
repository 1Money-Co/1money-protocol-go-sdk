package onemoney_test

import (
	"fmt"

	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
)

// submissionOutcome applies the submission rule: decide on the action only for
// a known type and a known action, never on the message. Anything else means
// check_status.
func submissionOutcome(p onemoney.Problem) string {
	if !p.IsKnownType() {
		return "unknown: look up the transaction before you sign anything new"
	}
	switch p.Action {
	case onemoney.ActionInProgress:
		hash, _ := p.TxHash()
		return "locked: resend the same signed transaction " + hash
	case onemoney.ActionFixRequest, onemoney.ActionRetryLater, onemoney.ActionUseOtherNode:
		return "rejected before admission"
	default:
		return "unknown: look up the transaction before you sign anything new"
	}
}

func ExampleProblem_IsKnownType() {
	inFlight := onemoney.ParseProblem([]byte(`{"type":"https://developer.1moneynetwork.com/problem/submission_already_in_flight",` +
		`"action":"in_progress","tx_hash":"0xabc","message":"already inflight"}`))
	fromTheFuture := onemoney.ParseProblem([]byte(`{"type":"https://developer.1moneynetwork.com/problem/from_the_future",` +
		`"action":"retry_later"}`))

	fmt.Println(submissionOutcome(inFlight))
	fmt.Println(submissionOutcome(fromTheFuture))
	// Output:
	// locked: resend the same signed transaction 0xabc
	// unknown: look up the transaction before you sign anything new
}
