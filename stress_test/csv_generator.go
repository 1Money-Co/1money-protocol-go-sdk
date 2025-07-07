package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

// generateAccountsDetailCSV generates a CSV file with account details for all wallet tiers
func (st *StressTester) generateAccountsDetailCSV(timestamp string) error {
	csvFileName := fmt.Sprintf("accounts_detail_%s.csv", timestamp)

	// Create CSV file
	file, err := os.Create(csvFileName)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header with additional columns for multi-tier tracking
	header := []string{"privatekey", "token_address", "decimal", "balance", "wallet_tier", "wallet_index", "source_wallet"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	log.Printf("Generating multi-tier accounts detail CSV file: %s", csvFileName)
	totalWallets := len(st.transferWallets) + len(st.distributionWallets)
	log.Printf("Collecting balance information for %d total wallets (%d primary + %d distribution)...",
		totalWallets, len(st.transferWallets), len(st.distributionWallets))

	processedCount := 0

	// Write data for primary transfer wallets (tier 2)
	log.Printf("Processing primary transfer wallets...")
	for i, wallet := range st.transferWallets {
		// Apply rate limiting for GET request
		if err := st.getRateLimiter.Wait(st.ctx); err != nil {
			log.Printf("Rate limiting failed for GetTokenAccount (primary wallet %d): %v", i+1, err)
			continue
		}

		// Get token account balance
		tokenAccount, err := st.client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		if err != nil {
			log.Printf("Warning: Failed to get balance for primary wallet %d (%s): %v", i+1, wallet.Address, err)
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Prepare CSV row for primary wallet
		row := []string{
			wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"primary",
			strconv.Itoa(i + 1),
			"mint_wallet", // Primary wallets are funded by mint wallets
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for primary wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every 200 wallets
		if processedCount%200 == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}

	// Write data for distribution wallets (tier 3)
	log.Printf("Processing distribution wallets...")
	for i, wallet := range st.distributionWallets {
		// Apply rate limiting for GET request
		if err := st.getRateLimiter.Wait(st.ctx); err != nil {
			log.Printf("Rate limiting failed for GetTokenAccount (distribution wallet %d): %v", i+1, err)
			continue
		}

		// Get token account balance
		tokenAccount, err := st.client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		if err != nil {
			log.Printf("Warning: Failed to get balance for distribution wallet %d (%s): %v", i+1, wallet.Address, err)
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Calculate which primary wallet funded this distribution wallet
		primaryWalletIndex := i / TRANSFER_MULTIPLIER
		sourceWallet := fmt.Sprintf("primary_wallet_%d", primaryWalletIndex+1)

		// Prepare CSV row for distribution wallet
		row := []string{
			wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"distribution",
			strconv.Itoa(i + 1),
			sourceWallet,
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for distribution wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every 500 wallets for distribution tier
		if processedCount%500 == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}

	log.Printf("Successfully generated multi-tier accounts detail CSV: %s", csvFileName)
	log.Printf("CSV contains %d total account records (%d primary + %d distribution)",
		totalWallets, len(st.transferWallets), len(st.distributionWallets))

	return nil
}
