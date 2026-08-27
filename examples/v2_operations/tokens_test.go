package v2operations_test

import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"

func Example_issue_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().Issue(e.ctx, e.issuePayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_issue_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().Issue(
		e.ctx,
		e.issuePayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_mint_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().Mint(e.ctx, e.mintPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_mint_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().Mint(
		e.ctx,
		e.mintPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_burn_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().Burn(e.ctx, e.burnPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_burn_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().Burn(
		e.ctx,
		e.burnPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_bridgeAndMint_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().BridgeAndMint(e.ctx, e.bridgeAndMintPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_bridgeAndMint_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().BridgeAndMint(
		e.ctx,
		e.bridgeAndMintPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_burnAndBridge_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().BurnAndBridge(e.ctx, e.burnAndBridgePayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_burnAndBridge_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().BurnAndBridge(
		e.ctx,
		e.burnAndBridgePayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_grantAuthority_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().GrantAuthority(e.ctx, e.authorityPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_grantAuthority_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().GrantAuthority(
		e.ctx,
		e.authorityPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_revokeAuthority_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().RevokeAuthority(e.ctx, e.authorityPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_revokeAuthority_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().RevokeAuthority(
		e.ctx,
		e.authorityPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_clawback_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().Clawback(e.ctx, e.clawbackPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_clawback_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().Clawback(
		e.ctx,
		e.clawbackPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_manageBlacklist_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().ManageBlacklist(e.ctx, e.manageListPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_manageBlacklist_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().ManageBlacklist(
		e.ctx,
		e.manageListPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_manageWhitelist_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().ManageWhitelist(e.ctx, e.manageListPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_manageWhitelist_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().ManageWhitelist(
		e.ctx,
		e.manageListPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_pause_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().Pause(e.ctx, e.pausePayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_pause_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().Pause(
		e.ctx,
		e.pausePayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_unpause_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().Unpause(e.ctx, e.pausePayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_unpause_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().Unpause(
		e.ctx,
		e.pausePayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}

func Example_updateMetadata_withoutMemo() {
	e := newExampleEnvironment()

	// Omitting WithMemo signs and submits the canonical EmptyMemo by default.
	if _, err := e.client.Tokens().UpdateMetadata(e.ctx, e.updateMetadataPayload(), e.signer); err != nil {
		panic(err)
	}
}

func Example_updateMetadata_withMemo() {
	e := newExampleEnvironment()

	if _, err := e.client.Tokens().UpdateMetadata(
		e.ctx,
		e.updateMetadataPayload(),
		e.signer,
		onemoney.WithMemo(exampleMemo()),
	); err != nil {
		panic(err)
	}
}
