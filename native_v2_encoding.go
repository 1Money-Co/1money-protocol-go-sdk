package onemoney

// This file defines the canonical RLP field layout (rlpList) for every native
// payload used in domain-separated v2 signing, plus the frozen operation-type
// mapping. Field order matches native-v2-signing-spec §4.2 exactly and is
// validated byte-for-byte against the L1 golden vectors
// (native_v2_conformance_test.go, native_v2_payload_test.go).

// pubkeyAsByteList encodes a compressed public key as a list of individually
// RLP-encoded single-byte integers (native-v2-signing-spec §3), the special
// form used only for MultiSigSigner.public_key inside CreateMultiSigPayload.
func pubkeyAsByteList(pk []byte) []interface{} {
	out := make([]interface{}, len(pk))
	for i, b := range pk {
		out[i] = uint64(b)
	}
	return out
}

func (p PaymentPayload) rlpList() []interface{} {
	return []interface{}{p.ChainID, p.Nonce, p.Recipient, bigOrZero(p.Value), p.Token}
}

func (p TokenIssuePayload) rlpList() []interface{} {
	return []interface{}{
		p.ChainID, p.Nonce, []byte(p.Symbol), []byte(p.Name), uint64(p.Decimals),
		p.MasterAuthority, boolToUint(p.IsPrivate), boolToUint(p.ClawbackEnabled),
	}
}

func (p TokenMintPayload) rlpList() []interface{} {
	return []interface{}{p.ChainID, p.Nonce, p.Recipient, bigOrZero(p.Value), p.Token}
}

func (p TokenAuthorityPayload) rlpList() []interface{} {
	return []interface{}{
		p.ChainID, p.Nonce, []byte(p.Action), []byte(p.AuthorityType),
		p.AuthorityAddress, p.Token, bigOrZero(p.Value),
	}
}

func (p TokenManageListPayload) rlpList() []interface{} {
	return []interface{}{p.ChainID, p.Nonce, []byte(p.Action), p.Address, p.Token}
}

func (p PauseTokenPayload) rlpList() []interface{} {
	return []interface{}{p.ChainID, p.Nonce, []byte(p.Action), p.Token}
}

func (p TokenBurnPayload) rlpList() []interface{} {
	return []interface{}{p.ChainID, p.Nonce, bigOrZero(p.Value), p.Token}
}

func (p TokenClawbackPayload) rlpList() []interface{} {
	return []interface{}{p.ChainID, p.Nonce, p.Token, p.From, p.Recipient, bigOrZero(p.Value)}
}

func (p UpdateMetadataPayload) rlpList() []interface{} {
	md := make([]interface{}, 0, len(p.AdditionalMetadata))
	for _, kv := range p.AdditionalMetadata {
		md = append(md, []interface{}{[]byte(kv.Key), []byte(kv.Value)})
	}
	return []interface{}{p.ChainID, p.Nonce, []byte(p.Name), []byte(p.URI), p.Token, md}
}

func (p TokenBridgeAndMintPayload) rlpList() []interface{} {
	return []interface{}{
		p.ChainID, p.Nonce, p.Recipient, bigOrZero(p.Value), p.Token,
		p.SourceChainID, []byte(p.SourceTxHash), []byte(p.BridgeMetadata),
	}
}

func (p TokenBurnAndBridgePayload) rlpList() []interface{} {
	return []interface{}{
		p.ChainID, p.Nonce, p.Sender, bigOrZero(p.Value), p.Token,
		p.DestinationChainID, []byte(p.DestinationAddress), bigOrZero(p.EscrowFee),
		[]byte(p.BridgeMetadata), []byte(p.BridgeParam),
	}
}

func (p CreateMultiSigPayload) rlpList() []interface{} {
	signers := make([]interface{}, 0, len(p.Signers))
	for _, s := range p.Signers {
		signers = append(signers, []interface{}{pubkeyAsByteList(s.PublicKey), uint64(s.Weight)})
	}
	return []interface{}{p.ChainID, p.Nonce, signers, uint64(p.Threshold)}
}

func (p BatchPaymentPayload) rlpList() []interface{} {
	ops := make([]interface{}, 0, len(p.Operations))
	for _, o := range p.Operations {
		ops = append(ops, []interface{}{o.Recipient, bigOrZero(o.Amount)})
	}
	list := []interface{}{p.ChainID, p.Nonce, p.Token, ops, bigOrZero(p.MaxFee), p.CreatedAt}
	// Trailing optional fields (native-v2-signing-spec §4.3): only appended when
	// present; an absent field before a present one becomes an empty placeholder.
	if p.OperationsHash != nil || p.BatchID != nil {
		if p.OperationsHash != nil {
			list = append(list, p.OperationsHash.Bytes())
		} else {
			list = append(list, []byte{})
		}
		if p.BatchID != nil {
			list = append(list, []byte(*p.BatchID))
		}
	}
	return list
}
