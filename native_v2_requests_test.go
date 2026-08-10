package onemoney

import "testing"

// TestPathsForOp locks pathsForOp's v1/v2-capability signal for every
// canonical operation type. submitPayload's v2-only guard
// (native_v2_requests.go) keys off op.pathV1 == "" alone, so a typo that
// emptied an unrelated operation's v1 path (e.g. opTokenMint) would silently
// reclassify a v1-capable operation as v2-only, and nothing else in the
// suite would catch it.
func TestPathsForOp(t *testing.T) {
	allOps := []nativeOperationType{
		opPayment, opTokenIssue, opTokenMint, opTokenAuthority, opTokenBlacklist,
		opTokenWhitelist, opTokenPause, opTokenBurn, opTokenClawback,
		opTokenMetadata, opTokenBridgeAndMint, opTokenBurnAndBridge,
		opCreateMultiSig, opBatchPayment,
	}
	if len(allOps) != 14 {
		t.Fatalf("test table lists %d ops, want all 14 canonical operation types", len(allOps))
	}

	v2Only := map[nativeOperationType]bool{
		opBatchPayment:   true,
		opCreateMultiSig: true,
	}

	for _, op := range allOps {
		v1, v2 := pathsForOp(op)
		if v2 == "" {
			t.Errorf("op %d: v2 path is empty, want non-empty", op)
		}
		wantV1Empty := v2Only[op]
		gotV1Empty := v1 == ""
		if gotV1Empty != wantV1Empty {
			t.Errorf("op %d: v1 path empty = %v (v1=%q), want empty = %v", op, gotV1Empty, v1, wantV1Empty)
		}
	}
}
