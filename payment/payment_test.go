package payment

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"go-1money/sign"
	"math/big"
	"testing"
)

var privateKey = "01833a126ec45d0191519748146b9e35647aab7fed28de1c8e17824970f964a3"

func TestPayment(t *testing.T) {
	tokenAddr := common.HexToAddress("0x2045a425D0e131E747f8be2F044413733e412d7d")
	payment := &Message{
		ChainID: big.NewInt(1212101),
		Nonce:   0,
		To:      common.HexToAddress("0x937b9aff6404141681cbf39301aeb869500bbdf0"),
		Value:   big.NewInt(1),
		Token:   &tokenAddr,
	}
	signature, err := sign.Message(payment, privateKey)
	if err != nil {
		t.Errorf(fmt.Sprintf("sign payment msg error: %v", err))
	}
	t.Logf("Payment msg Signature: %v\n", signature)
}
