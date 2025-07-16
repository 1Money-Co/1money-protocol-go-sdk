package main

import (
	"fmt"
	"log"
	"time"
)

// LogFunc is a function type for logging messages
type LogFunc func(format string, args ...interface{})

// runCompleteStressTestLegacy executes the complete stress test workflow (single node - legacy compatibility)
func runCompleteStressTestLegacy(logToFile LogFunc, fileLogger *log.Logger) error {
	// Use default testnet node
	defaultNodeURL := "https://testapi.1moneynetwork.com"
	nodeURLs := []string{defaultNodeURL}

	return runCompleteStressTest(logToFile, fileLogger, nodeURLs, POST_RATE_LIMIT_TPS, GET_RATE_LIMIT_TPS)
}

// runCompleteStressTestMultiNode executes the complete stress test workflow with multi-node support
func runCompleteStressTest(logToFile LogFunc, fileLogger *log.Logger, nodeURLs []string, totalPostRate int, totalGetRate int) error {
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

	// Log multi-node configuration
	logToFile("=== MULTI-NODE CONFIGURATION ===")
	logToFile("Number of nodes: %d", len(nodeURLs))
	for i, url := range nodeURLs {
		logToFile("Node %d: %s", i+1, url)
	}
	logToFile("Total POST rate: %d TPS (%.2f TPS per node)", totalPostRate, float64(totalPostRate)/float64(len(nodeURLs)))
	logToFile("Total GET rate: %d TPS (%.2f TPS per node)", totalGetRate, float64(totalGetRate)/float64(len(nodeURLs)))
	logToFile("")

	// Record tester creation time
	testerStartTime := time.Now()
	tester, err := NewStressTester(nodeURLs, totalPostRate, totalGetRate)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to create multi-node stress tester: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}
	testerCreationDuration := time.Since(testerStartTime)

	// Record stress test execution time
	stressTestStartTime := time.Now()
	if err := tester.RunStressTest(); err != nil {
		errorMsg := fmt.Sprintf("Multi-node batch mint stress test failed: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}
	stressTestDuration := time.Since(stressTestStartTime)

	logToFile("Multi-node batch mint stress test completed successfully!")

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
	logToFile("POST Rate Limit: %d TPS (total)", totalPostRate)
	logToFile("GET Rate Limit: %d TPS (total)", totalGetRate)
	logToFile("")

	// Multi-node efficiency analysis
	setupTime := testerCreationDuration
	executionTime := stressTestDuration
	csvTime := csvGenerationDuration
	setupPercentage := (setupTime.Seconds() / totalDuration.Seconds()) * 100
	executionPercentage := (executionTime.Seconds() / totalDuration.Seconds()) * 100
	csvPercentage := (csvTime.Seconds() / totalDuration.Seconds()) * 100

	logToFile("=== MULTI-NODE EFFICIENCY ANALYSIS ===")
	logToFile("Setup Time: %v (%.1f%% of total)", setupTime, setupPercentage)
	logToFile("Execution Time: %v (%.1f%% of total)", executionTime, executionPercentage)
	logToFile("CSV Generation Time: %v (%.1f%% of total)", csvTime, csvPercentage)
	logToFile("Combined Throughput: %.2f operations/minute", float64(totalOperations)/(stressTestDuration.Minutes()))
	logToFile("Mint Throughput: %.2f mint operations/minute", float64(totalMintOperations)/(stressTestDuration.Minutes()))
	logToFile("Transfer Throughput: %.2f transfer operations/minute", float64(totalTransferOperations)/(stressTestDuration.Minutes()))
	logToFile("Token Distribution Efficiency: %.2f tokens/second", float64(totalTokensMinted+totalTokensTransferred)/stressTestDuration.Seconds())
	logToFile("Multi-tier Concurrency Factor: %.2fx", float64(totalOperations)/float64(totalMintOperations))
	logToFile("Node Count Multiplier: %dx", len(nodeURLs))
	logToFile("Effective Operations per Node: %.2f ops/second/node", float64(totalOperations)/stressTestDuration.Seconds()/float64(len(nodeURLs)))
	logToFile("=== END OF TIMING STATISTICS ===")

	return nil
}

// RunStressTestMultiNode executes the complete stress test workflow with multi-node support
func (st *StressTester) RunStressTest() error {
	log.Println("=== Starting 1Money Multi-Tier Batch Mint Stress Test ===")
	log.Printf("Configuration:")
	log.Printf("- Node count: %d", st.nodePool.Size())
	log.Printf("- Mint wallets: %d", MINT_WALLETS_COUNT)
	log.Printf("- Primary transfer wallets: %d", TRANSFER_WALLETS_COUNT)
	log.Printf("- Distribution wallets: %d", DISTRIBUTION_WALLETS_COUNT)
	log.Printf("- Transfer multiplier: %d", TRANSFER_MULTIPLIER)
	log.Printf("- Transfer workers: %d", TRANSFER_WORKERS_COUNT)
	log.Printf("- Wallets per mint: %d", WALLETS_PER_MINT)
	log.Printf("- Mint allowance: %d", MINT_ALLOWANCE)
	log.Printf("- Mint amount per operation: %d", MINT_AMOUNT)
	log.Printf("- Transfer amount per operation: %d", TRANSFER_AMOUNT)
	log.Printf("- Token symbol: %s", GetTokenSymbol())
	log.Printf("- Token name: %s", TOKEN_NAME)
	log.Printf("- Chain ID: %d", CHAIN_ID)
	log.Println()

	// Step 1: Create mint wallets
	if err := st.createMintWallets(); err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}
	log.Println("✓ Step 1: Mint wallets created")

	// Step 2: Create primary transfer wallets
	if err := st.createTransferWallets(); err != nil {
		return fmt.Errorf("step 2 failed: %w", err)
	}
	log.Println("✓ Step 2: Primary transfer wallets created")

	// Step 2b: Create distribution wallets
	if err := st.createDistributionWallets(); err != nil {
		return fmt.Errorf("step 2b failed: %w", err)
	}
	log.Println("✓ Step 2b: Distribution wallets created")

	// Step 3: Create token (multi-node)
	if err := st.createToken(); err != nil {
		return fmt.Errorf("step 3 failed: %w", err)
	}
	log.Println("✓ Step 3: Token created")

	// Step 4: Grant mint authorities (multi-node)
	if err := st.grantMintAuthorities(); err != nil {
		return fmt.Errorf("step 4 failed: %w", err)
	}
	log.Println("✓ Step 4: Mint authorities granted")

	// Step 5: Perform concurrent minting with multi-tier transfers (multi-node)
	if err := st.performConcurrentMinting(); err != nil {
		return fmt.Errorf("step 5 failed: %w", err)
	}
	log.Println("✓ Step 5: Concurrent minting and transfers completed")

	log.Println("=== Multi-Tier Stress Test Completed Successfully! ===")
	log.Printf("Token Address: %s", st.tokenAddress)
	log.Printf("Total mint operations: %d", MINT_WALLETS_COUNT*WALLETS_PER_MINT)
	log.Printf("Total transfer operations: %d", TRANSFER_WALLETS_COUNT*TRANSFER_MULTIPLIER)
	log.Printf("Total tokens minted: %d", MINT_WALLETS_COUNT*WALLETS_PER_MINT*MINT_AMOUNT)
	log.Printf("Total tokens distributed: %d", TRANSFER_WALLETS_COUNT*TRANSFER_MULTIPLIER*TRANSFER_AMOUNT)

	return nil
}
