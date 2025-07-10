package main

import (
	"fmt"
	"log"
	"time"
)

// waitForTransactionReceiptWithContext waits for transaction receipt with detailed context information
func (st *StressTester) waitForTransactionReceiptWithContext(txHash, fromWallet, toWallet, txType string) error {
	timeout := time.After(RECEIPT_CHECK_TIMEOUT)
	ticker := time.NewTicker(RECEIPT_CHECK_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			errorMsg := fmt.Sprintf("TIMEOUT: Transaction receipt timeout after %v for txHash: %s", RECEIPT_CHECK_TIMEOUT, txHash)
			if fromWallet != "" {
				errorMsg += fmt.Sprintf(", From Wallet: %s", fromWallet)
			}
			if toWallet != "" {
				errorMsg += fmt.Sprintf(", To Wallet: %s", toWallet)
			}
			if txType != "" {
				errorMsg += fmt.Sprintf(", Transaction Type: %s", txType)
			}
			log.Printf("ERROR: %s", errorMsg)
			return fmt.Errorf("%s", errorMsg)
		case <-ticker.C:
			// Apply rate limiting for GET request
			if err := st.getRateLimiter.Wait(st.ctx); err != nil {
				log.Printf("Rate limiting failed for GetTransactionReceipt: %v", err)
				continue
			}

			receipt, err := st.client.GetTransactionReceipt(st.ctx, txHash)
			if err != nil {
				log.Printf("Error getting receipt for transaction %s: %v", txHash, err)
				continue
			}

			if receipt != nil {
				if receipt.Success {
					successMsg := fmt.Sprintf("Transaction %s confirmed successfully", txHash)
					if fromWallet != "" && toWallet != "" {
						successMsg += fmt.Sprintf(" (From: %s -> To: %s)", fromWallet, toWallet)
					}
					if txType != "" {
						successMsg += fmt.Sprintf(" [%s]", txType)
					}
					log.Printf("SUCCESS: %s", successMsg)
					return nil
				} else {
					failMsg := fmt.Sprintf("transaction %s failed", txHash)
					if fromWallet != "" {
						failMsg += fmt.Sprintf(", From Wallet: %s", fromWallet)
					}
					if toWallet != "" {
						failMsg += fmt.Sprintf(", To Wallet: %s", toWallet)
					}
					if txType != "" {
						failMsg += fmt.Sprintf(", Transaction Type: %s", txType)
					}
					log.Printf("FAILED: %s", failMsg)
					return fmt.Errorf("%s", failMsg)
				}
			}
		}
	}
}

// validateNonceIncrementWithContext validates nonce increment with detailed context information
func (st *StressTester) validateNonceIncrementWithContext(address string, expectedNonce uint64, walletType, txType string) error {
	timeout := time.After(NONCE_VALIDATION_TIMEOUT)
	ticker := time.NewTicker(NONCE_CHECK_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			errorMsg := fmt.Sprintf("NONCE_TIMEOUT: Nonce validation timeout after %v for address: %s, expected nonce: %d",
				NONCE_VALIDATION_TIMEOUT, address, expectedNonce)
			if walletType != "" {
				errorMsg += fmt.Sprintf(", Wallet Type: %s", walletType)
			}
			if txType != "" {
				errorMsg += fmt.Sprintf(", Transaction Type: %s", txType)
			}
			log.Printf("ERROR: %s", errorMsg)
			return fmt.Errorf("%s", errorMsg)
		case <-ticker.C:
			// Apply rate limiting for GET request
			if err := st.getRateLimiter.Wait(st.ctx); err != nil {
				log.Printf("Rate limiting failed for GetAccountNonce in nonce validation: %v", err)
				continue
			}

			accountNonce, err := st.client.GetAccountNonce(st.ctx, address)
			if err != nil {
				log.Printf("Error getting nonce for address %s: %v", address, err)
				continue
			}

			if accountNonce.Nonce == expectedNonce {
				successMsg := fmt.Sprintf("Nonce validated for address %s: %d", address, accountNonce.Nonce)
				if walletType != "" && txType != "" {
					successMsg += fmt.Sprintf(" [%s - %s]", walletType, txType)
				}
				log.Printf("NONCE_SUCCESS: %s", successMsg)
				return nil
			}

			if accountNonce.Nonce > expectedNonce {
				errorMsg := fmt.Sprintf("nonce validation failed for address %s: expected %d, got %d", address, expectedNonce, accountNonce.Nonce)
				if walletType != "" {
					errorMsg += fmt.Sprintf(", Wallet Type: %s", walletType)
				}
				if txType != "" {
					errorMsg += fmt.Sprintf(", Transaction Type: %s", txType)
				}
				log.Printf("NONCE_ERROR: %s", errorMsg)
				return fmt.Errorf("%s", errorMsg)
			}
		}
	}
}

// getAccountNonce retrieves the current nonce for an address
func (st *StressTester) getAccountNonce(address string) (uint64, error) {
	// Apply rate limiting for GET request
	if err := st.getRateLimiter.Wait(st.ctx); err != nil {
		return 0, fmt.Errorf("rate limiting failed for GetAccountNonce: %w", err)
	}

	accountNonce, err := st.client.GetAccountNonce(st.ctx, address)
	if err != nil {
		return 0, fmt.Errorf("failed to get account nonce for %s: %w", address, err)
	}
	return accountNonce.Nonce, nil
}
