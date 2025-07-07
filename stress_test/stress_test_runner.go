package main

import (
	"fmt"
	"log"
	"time"
)

// LogFunc is a function type for logging messages
type LogFunc func(format string, args ...interface{})

// runCompleteStressTest executes the complete stress test workflow with custom logging
func runCompleteStressTest(logToFile LogFunc, fileLogger *log.Logger) error {
	// Record overall test start time
	overallStartTime := time.Now()

	// Validate configuration
	if MINT_WALLETS_COUNT*WALLETS_PER_MINT != TRANSFER_WALLETS_COUNT {
		errorMsg := fmt.Sprintf("Configuration error: MINT_WALLETS_COUNT (%d) * WALLETS_PER_MINT (%d) must equal TRANSFER_WALLETS_COUNT (%d)",
			MINT_WALLETS_COUNT, WALLETS_PER_MINT, TRANSFER_WALLETS_COUNT)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}

	if TRANSFER_WALLETS_COUNT*TRANSFER_MULTIPLIER != DISTRIBUTION_WALLETS_COUNT {
		errorMsg := fmt.Sprintf("Configuration error: TRANSFER_WALLETS_COUNT (%d) * TRANSFER_MULTIPLIER (%d) must equal DISTRIBUTION_WALLETS_COUNT (%d)",
			TRANSFER_WALLETS_COUNT, TRANSFER_MULTIPLIER, DISTRIBUTION_WALLETS_COUNT)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}

	if MINT_AMOUNT%TRANSFER_MULTIPLIER != 0 {
		errorMsg := fmt.Sprintf("Configuration error: MINT_AMOUNT (%d) must be divisible by TRANSFER_MULTIPLIER (%d) for even distribution",
			MINT_AMOUNT, TRANSFER_MULTIPLIER)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}

	// Record tester creation time
	testerStartTime := time.Now()
	tester, err := NewStressTester()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to create stress tester: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}
	testerCreationDuration := time.Since(testerStartTime)

	// Record stress test execution time
	stressTestStartTime := time.Now()
	if err := tester.RunStressTest(); err != nil {
		errorMsg := fmt.Sprintf("Batch mint stress test failed: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}
	stressTestDuration := time.Since(stressTestStartTime)

	logToFile("Batch mint stress test completed successfully!")

	// Generate accounts detail CSV file
	timestamp := time.Now().Format("20060102_150405")
	csvStartTime := time.Now()
	logToFile("Generating accounts detail CSV file...")
	if err := tester.generateAccountsDetailCSV(timestamp); err != nil {
		errorMsg := fmt.Sprintf("Failed to generate accounts detail CSV: %v", err)
		fileLogger.Println("ERROR: " + errorMsg)
		logToFile("ERROR: %s", errorMsg) // Continue execution but report error
	} else {
		logToFile("Accounts detail CSV file generated successfully!")
	}
	csvGenerationDuration := time.Since(csvStartTime)

	// Calculate total test duration including CSV generation
	totalDuration := time.Since(overallStartTime)

	// Detailed timing statistics
	logToFile("=== DETAILED TIMING STATISTICS ===")
	logToFile("Tester Creation Time: %v", testerCreationDuration)
	logToFile("Stress Test Execution Time: %v", stressTestDuration)
	logToFile("CSV Generation Time: %v", csvGenerationDuration)
	logToFile("Total Test Duration: %v", totalDuration)
	logToFile("")

	// Performance metrics
	totalMintOperations := MINT_WALLETS_COUNT * WALLETS_PER_MINT
	totalTransferOperations := TRANSFER_WALLETS_COUNT * TRANSFER_MULTIPLIER
	totalOperations := totalMintOperations + totalTransferOperations
	totalTokensMinted := totalMintOperations * MINT_AMOUNT
	totalTokensTransferred := totalTransferOperations * TRANSFER_AMOUNT

	logToFile("=== PERFORMANCE METRICS ===")
	logToFile("Total Mint Operations: %d", totalMintOperations)
	logToFile("Total Transfer Operations: %d", totalTransferOperations)
	logToFile("Total Combined Operations: %d", totalOperations)
	logToFile("Total Tokens Minted: %d", totalTokensMinted)
	logToFile("Total Tokens Transferred: %d", totalTokensTransferred)
	logToFile("Combined Operations per Second: %.2f", float64(totalOperations)/stressTestDuration.Seconds())
	logToFile("Mint Operations per Second: %.2f", float64(totalMintOperations)/stressTestDuration.Seconds())
	logToFile("Transfer Operations per Second: %.2f", float64(totalTransferOperations)/stressTestDuration.Seconds())
	logToFile("Tokens Processed per Second: %.2f", float64(totalTokensMinted+totalTokensTransferred)/stressTestDuration.Seconds())
	logToFile("Average Time per Combined Operation: %v", time.Duration(int64(stressTestDuration)/int64(totalOperations)))
	logToFile("")

	// Configuration summary
	logToFile("=== TEST CONFIGURATION ===")
	logToFile("Mint Wallets Count: %d", MINT_WALLETS_COUNT)
	logToFile("Primary Transfer Wallets Count: %d", TRANSFER_WALLETS_COUNT)
	logToFile("Distribution Wallets Count: %d", DISTRIBUTION_WALLETS_COUNT)
	logToFile("Transfer Multiplier: %d", TRANSFER_MULTIPLIER)
	logToFile("Transfer Workers Count: %d", TRANSFER_WORKERS_COUNT)
	logToFile("Wallets per Mint: %d", WALLETS_PER_MINT)
	logToFile("Mint Amount per Operation: %d", MINT_AMOUNT)
	logToFile("Transfer Amount per Operation: %d", TRANSFER_AMOUNT)
	logToFile("Token Symbol: %s", GetTokenSymbol())
	logToFile("Chain ID: %d", CHAIN_ID)
	logToFile("POST Rate Limit: %d TPS", POST_RATE_LIMIT_TPS)
	logToFile("GET Rate Limit: %d TPS", GET_RATE_LIMIT_TPS)
	logToFile("")

	// Efficiency analysis
	setupTime := testerCreationDuration
	executionTime := stressTestDuration
	csvTime := csvGenerationDuration
	setupPercentage := (setupTime.Seconds() / totalDuration.Seconds()) * 100
	executionPercentage := (executionTime.Seconds() / totalDuration.Seconds()) * 100
	csvPercentage := (csvTime.Seconds() / totalDuration.Seconds()) * 100

	logToFile("=== EFFICIENCY ANALYSIS ===")
	logToFile("Setup Time: %v (%.1f%% of total)", setupTime, setupPercentage)
	logToFile("Execution Time: %v (%.1f%% of total)", executionTime, executionPercentage)
	logToFile("CSV Generation Time: %v (%.1f%% of total)", csvTime, csvPercentage)
	logToFile("Combined Throughput: %.2f operations/minute", float64(totalOperations)/(stressTestDuration.Minutes()))
	logToFile("Mint Throughput: %.2f mint operations/minute", float64(totalMintOperations)/(stressTestDuration.Minutes()))
	logToFile("Transfer Throughput: %.2f transfer operations/minute", float64(totalTransferOperations)/(stressTestDuration.Minutes()))
	logToFile("Token Distribution Efficiency: %.2f tokens/second", float64(totalTokensMinted+totalTokensTransferred)/stressTestDuration.Seconds())
	logToFile("Multi-tier Concurrency Factor: %.2fx", float64(totalOperations)/float64(totalMintOperations))
	logToFile("=== END OF TIMING STATISTICS ===")

	return nil
}
