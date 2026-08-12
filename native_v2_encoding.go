package onemoney

import "math/big"

// This file holds both serializations of every native-v2 payload, kept together
// because they change in lockstep whenever a payload is added or edited:
//
//   - rlpList: the canonical RLP field layout used as the domain-separated v2
//     signing input. Field order matches native-v2-signing-spec §4.2 exactly and
//     is validated byte-for-byte against the L1 golden vectors
//     (native_v2_conformance_test.go, native_v2_payload_test.go).
//   - wireFields: the flattened JSON body for the /v2 REST request. The server
//     flattens its signing payload into the request object, so the keys match the
//     L1 payload field names exactly. U256 amounts are decimal strings, addresses
//     marshal to 0x-hex, and chain_id/nonce/decimals/weight/threshold stay numeric.

// -----------------------------------------------------------------------------
// Canonical RLP field layout (v2 signing input)
// -----------------------------------------------------------------------------

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
	list := []interface{}{p.ChainID, p.Nonce, p.Token, batchOperationsRLPList(p.Operations), p.CreatedAt}
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

// -----------------------------------------------------------------------------
// Flattened JSON body fields (v2 REST request)
// -----------------------------------------------------------------------------

func bigStr(v *big.Int) string { return bigOrZero(v).String() }

func (p PaymentPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "recipient": p.Recipient,
		"value": bigStr(p.Value), "token": p.Token,
	}
}

func (p TokenIssuePayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "symbol": p.Symbol, "name": p.Name,
		"decimals": p.Decimals, "master_authority": p.MasterAuthority,
		"is_private": p.IsPrivate, "clawback_enabled": p.ClawbackEnabled,
	}
}

func (p TokenMintPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "recipient": p.Recipient,
		"value": bigStr(p.Value), "token": p.Token,
	}
}

func (p TokenAuthorityPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "action": p.Action,
		"authority_type": p.AuthorityType, "authority_address": p.AuthorityAddress,
		"token": p.Token, "value": bigStr(p.Value),
	}
}

func (p TokenManageListPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "action": p.Action,
		"address": p.Address, "token": p.Token,
	}
}

func (p PauseTokenPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "action": p.Action, "token": p.Token,
	}
}

func (p TokenBurnPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "value": bigStr(p.Value), "token": p.Token,
	}
}

func (p TokenClawbackPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "token": p.Token, "from": p.From,
		"recipient": p.Recipient, "value": bigStr(p.Value),
	}
}

func (p UpdateMetadataPayload) wireFields() map[string]interface{} {
	// Copy the slice so the assembled body never aliases the caller's payload
	// (elements are value structs of immutable strings, so a slice copy suffices).
	metadata := append([]AdditionalMetadata(nil), p.AdditionalMetadata...)
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "name": p.Name, "uri": p.URI,
		"token": p.Token, "additional_metadata": metadata,
	}
}

func (p TokenBridgeAndMintPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "recipient": p.Recipient,
		"value": bigStr(p.Value), "token": p.Token, "source_chain_id": p.SourceChainID,
		"source_tx_hash": p.SourceTxHash, "bridge_metadata": p.BridgeMetadata,
	}
}

func (p TokenBurnAndBridgePayload) wireFields() map[string]interface{} {
	// Copy the bytes so the body never aliases the caller's slice.
	bridgeParam := append(HexBytes(nil), p.BridgeParam...)
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "sender": p.Sender, "value": bigStr(p.Value),
		"token": p.Token, "destination_chain_id": p.DestinationChainID,
		"destination_address": p.DestinationAddress, "escrow_fee": bigStr(p.EscrowFee),
		"bridge_metadata": p.BridgeMetadata, "bridge_param": bridgeParam,
	}
}

func (p CreateMultiSigPayload) wireFields() map[string]interface{} {
	signers := make([]map[string]interface{}, 0, len(p.Signers))
	for _, s := range p.Signers {
		// The L1 payload field is a bare Vec<u8>, which serde serializes as a
		// JSON number array ([2,17,...]). A Go []byte/[]uint8 would marshal as
		// base64, so convert to a non-byte integer slice.
		pk := make([]uint16, len(s.PublicKey))
		for i, b := range s.PublicKey {
			pk[i] = uint16(b)
		}
		signers = append(signers, map[string]interface{}{"public_key": pk, "weight": s.Weight})
	}
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "signers": signers, "threshold": p.Threshold,
	}
}

func (p BatchPaymentPayload) wireFields() map[string]interface{} {
	body := map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "token": p.Token,
		"operations": batchOperationsWireList(p.Operations), "created_at": p.CreatedAt,
	}
	if p.OperationsHash != nil {
		// Store a value copy so the body never aliases the caller's pointer.
		body["operations_hash"] = *p.OperationsHash
	}
	if p.BatchID != nil {
		body["batch_id"] = *p.BatchID
	}
	return body
}
