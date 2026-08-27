package v2operations_test

import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"

func Example_payment_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Transactions().Payment(e.ctx, e.paymentPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_payment_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Transactions().Payment(
		e.ctx,
		e.paymentPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_batchPayment_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Transactions().BatchPayment(e.ctx, e.batchPaymentPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_batchPayment_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Transactions().BatchPayment(
		e.ctx,
		e.batchPaymentPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}
