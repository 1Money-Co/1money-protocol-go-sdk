package payment

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type Message struct {
	ChainID *big.Int
	Nonce   uint64
	To      common.Address
	Value   *big.Int
	Token   *common.Address `rlp:"optional"`
}

type Signature struct {
	R string
	S string
	V uint64
}
