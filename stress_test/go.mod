module stress_test

go 1.24

toolchain go1.24.1

require (
	github.com/1Money-Co/1money-go-sdk v0.0.0
	github.com/ethereum/go-ethereum v1.15.7
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/time v0.12.0
)

replace github.com/1Money-Co/1money-go-sdk => ../
