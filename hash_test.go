package onemoney

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func Test_PaymentRequest_Hash(t *testing.T) {
	value := big.NewInt(0)
	value.SetString("10", 10)

	paymentTx := PaymentRequest{
		PaymentPayload: PaymentPayload{
			ChainID:   1212101,
			Nonce:     0,
			Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
			Value:     value,
			Token:     common.HexToAddress("0x5458747a0efb9ebeb8696fcac1479278c0872fbe"),
		},
		Signature: Signature{
			R: "29799431026396113297345952769532737771367335026226509821050116192126323991602",
			S: "15357736211266391569611566560819218221258872050529851723622905759192743831009",
			V: 0,
		},
	}

	txHash, err := paymentTx.Hash()
	assert.Nil(t, err)
	assert.Equal(t, common.HexToHash("0xd002ef79e1b20b132d3bc679df4db240c891d5408c50b883f9020e9d65ac3740"), txHash)
}

func Test_PaymentRequest_Hash_withHexSignature(t *testing.T) {
	value := big.NewInt(0)
	value.SetString("10", 10)

	paymentTx := PaymentRequest{
		PaymentPayload: PaymentPayload{
			ChainID:   1212101,
			Nonce:     0,
			Recipient: common.HexToAddress("0xA634dfba8c7550550817898bC4820cD10888Aac5"),
			Value:     value,
			Token:     common.HexToAddress("0x5458747a0efb9ebeb8696fcac1479278c0872fbe"),
		},
		Signature: Signature{
			R: "0x41e1e158803da19ef1fc9ab35d86776cb02ac493265b948ff18b2c57a4e52432",
			S: "0x21f42bb02796a424b0961af374a71e0b948e8fadb58f1e5c6ac861be656265e1",
			V: 0,
		},
	}

	txHash, err := paymentTx.Hash()
	assert.Nil(t, err)
	assert.Equal(t, common.HexToHash("0xd002ef79e1b20b132d3bc679df4db240c891d5408c50b883f9020e9d65ac3740"), txHash)
}
