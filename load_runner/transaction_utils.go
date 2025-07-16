package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// HardcodedChainID is the fixed chain ID for 1Money network
	HardcodedChainID = 1212101
)

// TransactionResult represents the result of a transaction
type TransactionResult struct {
	AccountIndex      int
	WalletIndex       string
	FromAddress       string
	TxHash            string
	Success           bool
	Error             error
	Duration          time.Duration
	SendTime          time.Time
	ResponseTime      time.Time
	Verified          bool
	VerificationError error
	TxSuccess         bool
	NodeIndex         int
	NodeURL           string
	NodeCount         int64
}

// SendSingleTransactionToNode sends a single transaction to a specific node
func SendSingleTransactionToNode(
	client *onemoney.Client,
	nodeURL string,
	nodeIndex int,
	nodePool *BalancedNodePool,
	account Account,
	toAddress string,
	amount string,
) TransactionResult {
	startTime := time.Now()
	result := TransactionResult{
		WalletIndex: account.WalletIndex,
		NodeIndex:   nodeIndex,
		NodeURL:     nodePool.GetNodeURL(nodeIndex),
	}
	
	// Increment node count
	nodeCount := nodePool.IncrementNodeCount(nodeIndex)
	result.NodeCount = nodeCount
	
	// Parse private key
	privateKeyHex := strings.TrimPrefix(account.PrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to parse private key: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to cast public key to ECDSA")
		result.Duration = time.Since(startTime)
		return result
	}
	
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	result.FromAddress = fromAddress.Hex()
	
	// Prepare transaction
	amountBig := new(big.Int)
	amountBig.SetString(amount, 10)
	
	payload := onemoney.PaymentPayload{
		ChainID:   HardcodedChainID,
		Nonce:     uint64(0),
		Recipient: common.HexToAddress(toAddress),
		Value:     amountBig,
		Token:     common.HexToAddress(account.TokenAddress),
	}
	
	signature, err := client.SignMessage(payload, account.PrivateKey)
	if err != nil {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to sign payment: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	paymentReq := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature:      *signature,
	}
	
	// Send transaction
	ctx := context.Background()
	result.SendTime = time.Now()
	paymentResp, err := client.SendPayment(ctx, paymentReq)
	result.ResponseTime = time.Now()
	
	if err != nil {
		result.Error = fmt.Errorf("failed to send payment to %s: %w", nodeURL, err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	result.TxHash = paymentResp.Hash
	result.Success = true
	result.Duration = time.Since(startTime)
	return result
}

// VerifyTransaction verifies a transaction receipt
func VerifyTransaction(client *onemoney.Client, txHash string) (bool, error) {
	ctx := context.Background()
	receipt, err := client.GetTransactionReceipt(ctx, txHash)
	if err != nil {
		return false, err
	}
	return receipt.Success, nil
}