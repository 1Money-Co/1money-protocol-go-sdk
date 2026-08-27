package v2operations_test

import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"

func Example_createMultisig_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Accounts().CreateMultisig(e.ctx, e.createMultisigPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_createMultisig_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Accounts().CreateMultisig(
		e.ctx,
		e.createMultisigPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}
