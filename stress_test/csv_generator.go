package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

// generateAccountsDetailCSV generates a CSV file with account details for all wallet tiers (multi-node compatible)
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

	log.Printf("Generating accounts detail CSV file: %s", csvFileName)
	totalWallets := len(st.transferWallets)
	log.Printf("Collecting balance information for %d wallets...", totalWallets)

	processedCount := 0

	// Write data for transfer wallets
	log.Printf("Processing transfer wallets...")
	for i, wallet := range st.transferWallets {
		// Get a node for GET operation
		client, _, _, err := st.nodePool.GetNodeForGet()
		if err != nil {
			log.Printf("Failed to get node for balance check (wallet %d): %v", i+1, err)
			continue
		}

		// Skip rate limiting for CSV generation to speed up

		// Get token account balance
		tokenAccount, err := client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		if err != nil {
			// Failed to get balance
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Prepare CSV row for transfer wallet
		row := []string{
			"0x" + wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"transfer",
			strconv.Itoa(i + 1),
			"mint_wallet", // Transfer wallets are funded by mint wallets
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every CSV_PROGRESS_INTERVAL_WALLETS wallets
		if processedCount%CSV_PROGRESS_INTERVAL_WALLETS == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}


	log.Printf("✓ CSV generated: %s (%d entries)", csvFileName, processedCount)

	return nil
}
