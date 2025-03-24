package cancel

import (
	"math/big"
)

type Message struct {
	ChainID *big.Int
	Nonce   uint64
}

type Signature struct {
	R string
	S string
	V uint64
}
