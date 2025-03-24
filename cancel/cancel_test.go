package cancel

import (
	"fmt"
	"go-1money/sign"
	"math/big"
	"testing"
)

var privateKey = "01833a126ec45d0191519748146b9e35647aab7fed28de1c8e17824970f964a3"

func TestCancel(t *testing.T) {
	cancel := &Message{
		ChainID: big.NewInt(1212101),
		Nonce:   0,
	}
	signature, err := sign.Message(cancel, privateKey)
	if err != nil {
		t.Errorf(fmt.Sprintf("sign cancel msg error: %v", err))
	}
	t.Logf("Cancellation msg Signature: %v\n", signature)
}
