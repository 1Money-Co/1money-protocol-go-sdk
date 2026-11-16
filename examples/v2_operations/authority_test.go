package v2operations_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

// storeSigned writes each signed transaction before the SDK sends it, so that
// after any error the caller reads the stored transaction's outcome instead of
// signing the intent again. A real store must be durable (a database row, a
// file that is synced).
func storeSigned(_ context.Context, signed onemoney.SignedTransaction) error {
	record, err := json.Marshal(signed.Transaction)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("signed-%x.json", signed.Transaction.TransactionHash())
	return os.WriteFile(name, record, 0o600)
}

func Example_authorityOperationByMode() {
	e := newExampleEnvironment()

	// The legacy payload as before. The SDK reads the node's envelope mode: it
	// sends the legacy envelope before activation, nothing in the upgrade gap
	// (ErrUpgradeGap), and the settled envelope at the same nonce after it.
	pause := onemoney.LegacyPause(e.pausePayload())
	submission, err := e.client.Tokens().AuthorityOperationByMode(
		e.ctx, pause, e.signer, onemoney.WithOnSigned(storeSigned),
	)
	if err != nil {
		panic(err)
	}

	// The checkpoint decides a settled operation. Read its state until the
	// receipt decides it; a long pending or settling is not a failure.
	state, err := e.client.AuthorityOperationState(e.ctx, e.signer.Address(), e.nonce, submission.TransactionHash)
	if err != nil {
		panic(err)
	}
	switch state.Kind {
	case onemoney.AuthorityDecided:
		outcome := onemoney.ReceiptOutcomeOf(state.Receipt)
		if outcome.NoOpRevoke != nil {
			// The named account held no role; another account can still hold it.
			fmt.Println("warning: the revoke changed nothing")
		}
	case onemoney.AuthorityUnknown:
		// Never report it as failed, and never sign the intent again by itself.
		fmt.Println("unknown: check on a node with the full history")
	}
}

func Example_laneClawback_storeBeforeSend() {
	e := newExampleEnvironment()

	payload, err := e.client.LaneClawbackPayload(
		e.ctx, e.chainID,
		requiredAddress("TOKEN_ADDRESS"), requiredAddress("ACCOUNT_ADDRESS"),
		requiredAddress("RECIPIENT_ADDRESS"), big.NewInt(100),
	)
	if err != nil {
		panic(err)
	}
	prepared, err := onemoney.PrepareTransaction(payload)
	if err != nil {
		panic(err)
	}
	signature, err := e.signer.SignHash(prepared.SigningHash())
	if err != nil {
		panic(err)
	}
	authorized, err := prepared.Authorize(signature)
	if err != nil {
		panic(err)
	}
	record, err := onemoney.NewLaneClawbackRecord(authorized)
	if err != nil {
		panic(err)
	}

	// Store the record before the send. After any error, load it and read its
	// fate; never sign a new clawback because a send failed.
	if err := storeSigned(e.ctx, onemoney.SignedTransaction{Transaction: authorized}); err != nil {
		panic(err)
	}
	if _, err := e.client.Submit(e.ctx, authorized); err != nil {
		fmt.Println("send failed; read the stored clawback's fate")
	}
	fate, err := e.client.LaneClawbackFate(e.ctx, record)
	if err != nil {
		panic(err)
	}
	fmt.Println(fate.Kind)
}

func Example_clawbackByMode() {
	e := newExampleEnvironment()

	// One call per intent: after any error, load the last stored transaction
	// and read its outcome; never call ClawbackByMode again for this intent.
	submission, err := e.client.Tokens().ClawbackByMode(
		e.ctx, e.clawbackPayload(), onemoney.ClawbackFromAccount, e.signer, storeSigned,
	)
	if err != nil {
		panic(err)
	}
	if submission.Lane != nil {
		fate, err := e.client.LaneClawbackFate(e.ctx, submission.Lane)
		if err != nil {
			panic(err)
		}
		fmt.Println(fate.Kind)
	}
}

func Example_laneClawbackReissue() {
	e := newExampleEnvironment()

	stored, err := os.ReadFile(requiredEnvironment("DEAD_CLAWBACK_FILE"))
	if err != nil {
		panic(err)
	}
	dead, err := onemoney.ParseLaneClawbackRecord(stored)
	if err != nil {
		panic(err)
	}
	// Only after a node with the full history shows no receipt and no status
	// by hash for dead, and the holder's nonce past its slot.
	reissue, err := e.client.PrepareLaneClawbackReissue(e.ctx, dead, e.signer, onemoney.FullHistoryChecked)
	if err != nil {
		panic(err)
	}
	if reissue.Kind != onemoney.ReissuePrepared {
		fmt.Println("not issued again:", reissue.Kind, reissue.Fate.Kind)
		return
	}
	replacement := reissue.Replacement.Transaction()
	if err := storeSigned(e.ctx, onemoney.SignedTransaction{Transaction: replacement, Replaces: ptrHash(dead.TransactionHash())}); err != nil {
		panic(err)
	}
	if _, err := e.client.Submit(e.ctx, replacement); err != nil {
		fmt.Println("send failed; read the replacement's fate")
	}
}

func ptrHash(hash common.Hash) *common.Hash { return &hash }
