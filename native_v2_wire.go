package onemoney

import "math/big"

// This file builds the flattened JSON body fields for each native operation's
// /v2 request. The server flattens its signing payload into the request object,
// so the keys here match the L1 payload field names exactly. U256 amounts are
// serialized as decimal strings (the L1 DTOs deserialize `value` from a decimal
// string, e.g. "1000000000000000000"); addresses marshal to 0x-hex via
// common.Address, and chain_id/nonce/decimals/weight/threshold stay numeric.

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
	ops := make([]map[string]interface{}, 0, len(p.Operations))
	for _, o := range p.Operations {
		ops = append(ops, map[string]interface{}{"recipient": o.Recipient, "amount": bigStr(o.Amount)})
	}
	body := map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "token": p.Token, "operations": ops,
		"max_fee": bigStr(p.MaxFee), "created_at": p.CreatedAt,
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
