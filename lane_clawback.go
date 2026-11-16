package onemoney

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// LaneClawbackPayload builds a lane clawback at the holder's next nonce, with
// the latest committed checkpoint as its reference. Sign it only through a
// path that stores it before its send: a lane clawback that executed and one
// whose slot another transaction took read the same on a node without the full
// history, so a lost record can lead to a second debit of the holder.
//
// Errors: the VERIFY token, which has no lane clawback (its master claws it
// back with the legacy clawback); a holder whose next nonce is 0, which has no
// lane yet, so admission refuses the clawback (a Reserved Payment hold needs
// the hold clawback).
func (c *Client) LaneClawbackPayload(ctx context.Context, chainID uint64, token, holder, recipient common.Address, value *big.Int) (LaneClawbackPayload, error) {
	if err := refuseLaneClawbackOfVerify(token); err != nil {
		return LaneClawbackPayload{}, err
	}
	holderNonce, err := c.GetAccountNonce(ctx, holder)
	if err != nil {
		return LaneClawbackPayload{}, fmt.Errorf("holder nonce read: %w", err)
	}
	if holderNonce.Nonce == 0 {
		return LaneClawbackPayload{}, fmt.Errorf("holder %s has no lane yet (its next nonce is 0), so a lane clawback cannot claw it back; a Reserved Payment hold needs the hold clawback", holder.Hex())
	}
	reference, err := c.LatestReferenceCheckpoint(ctx)
	if err != nil {
		return LaneClawbackPayload{}, fmt.Errorf("reference checkpoint read: %w", err)
	}
	return LaneClawbackPayload{
		ChainID: chainID, Token: token, Holder: holder, FromNonce: holderNonce.Nonce,
		ReferenceCheckpoint: reference, Recipient: recipient, Value: value,
	}, nil
}
