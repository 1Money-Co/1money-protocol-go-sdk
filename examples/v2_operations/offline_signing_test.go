package v2operations_test

import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"

func Example_offlineSigning_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo prepares the transaction with the canonical EmptyMemo.
	prepared, err := onemoney.PrepareTransaction(e.paymentPayload())
	if err != nil {
		panic(err)
	}

	// Send this exact 32-byte digest to an external KMS, HSM, MPC, or signer.
	signature, err := e.signer.SignHash(prepared.SigningHash())
	if err != nil {
		panic(err)
	}

	authorized, err := prepared.Authorize(signature)
	if err != nil {
		panic(err)
	}
	transactionHash := authorized.TransactionHash()

	response, err := e.client.Submit(e.ctx, authorized)
	if err != nil {
		panic(err)
	}

	_, _ = transactionHash, response
}

func Example_offlineSigning_withMemo() {
	e := newExampleEnvironment()

	// The memo is included in the signing hash and carried into the authorized
	// request body by the PreparedTransaction.
	prepared, err := onemoney.PrepareTransaction(
		e.mintPayload(),
		onemoney.WithMemo(exampleMemo()),
	)
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
	transactionHash := authorized.TransactionHash()

	response, err := e.client.Submit(e.ctx, authorized)
	if err != nil {
		panic(err)
	}

	_, _ = transactionHash, response
}

func Example_offlineSigning_manageBlacklistWithMemo() {
	e := newExampleEnvironment()

	// TokenManageListPayload is shared by blacklist and whitelist operations.
	// WithManageListKind is therefore required when preparing it directly.
	prepared, err := onemoney.PrepareTransaction(
		e.manageListPayload(),
		onemoney.WithManageListKind(onemoney.ManageListBlacklist),
		onemoney.WithMemo(exampleMemo()),
	)
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
	transactionHash := authorized.TransactionHash()

	response, err := e.client.Submit(e.ctx, authorized)
	if err != nil {
		panic(err)
	}

	_, _ = transactionHash, response
}
