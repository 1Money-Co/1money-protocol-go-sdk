package main

import (
	"fmt"
	"log"
	"time"
)

// LogFunc is a function type for logging messages
type LogFunc func(format string, args ...interface{})


// runCompleteStressTest executes the complete stress test workflow
func runCompleteStressTest(logToFile LogFunc, fileLogger *log.Logger, nodeURLs []string, totalPostRate int, totalGetRate int, csvRateLimit int) error {
	// Record overall test start time
	overallStartTime := time.Now()

	// Validate configuration
	if MINT_WALLETS_COUNT*WALLETS_PER_MINT != TRANSFER_WALLETS_COUNT {
		errorMsg := fmt.Sprintf("Configuration error: MINT_WALLETS_COUNT (%d) * WALLETS_PER_MINT (%d) must equal TRANSFER_WALLETS_COUNT (%d)",
			MINT_WALLETS_COUNT, WALLETS_PER_MINT, TRANSFER_WALLETS_COUNT)
		fileLogger.Println("FATAL: " + errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}


	// Log node configuration
	logToFile("=== NODE CONFIGURATION ===")
	logToFile("Number of nodes: %d", len(nodeURLs))
	for i, url := range nodeURLs {
		logToFile("Node %d: %s", i+1, url)
	}
	logToFile("Total POST rate: %d TPS (%.2f TPS per node)", totalPostRate, float64(totalPostRate)/float64(len(nodeURLs)))
	logToFile("Total GET rate: %d TPS (%.2f TPS per node)", totalGetRate, float64(totalGetRate)/float64(len(nodeURLs)))
	logToFile("CSV balance query rate: %d QPS", csvRateLimit)
	logToFile("")

	// Record tester creation time
	testerStartTime := time.Now()
	tester, err := NewStressTester(nodeURLs, totalPostRate, totalGetRate, csvRateLimit)
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
	totalTokensMinted := totalMintOperations * MINT_AMOUNT

	logToFile("=== PERFORMANCE METRICS ===")
	logToFile("Total Mint Operations: %d", totalMintOperations)
	logToFile("Total Tokens Minted: %d", totalTokensMinted)
	logToFile("Mint Operations per Second: %.2f", float64(totalMintOperations)/stressTestDuration.Seconds())
	logToFile("Tokens Minted per Second: %.2f", float64(totalTokensMinted)/stressTestDuration.Seconds())
	logToFile("Average Time per Mint Operation: %v", time.Duration(int64(stressTestDuration)/int64(totalMintOperations)))
	logToFile("")

	// Configuration summary
	logToFile("=== TEST CONFIGURATION ===")
	logToFile("Mint Wallets Count: %d", MINT_WALLETS_COUNT)
	logToFile("Primary Transfer Wallets Count: %d", TRANSFER_WALLETS_COUNT)
	logToFile("Wallets per Mint: %d", WALLETS_PER_MINT)
	logToFile("Mint Amount per Operation: %d", MINT_AMOUNT)
	logToFile("Token Symbol: %s", GetTokenSymbol())
	logToFile("Chain ID: %d", CHAIN_ID)
	logToFile("POST Rate Limit: %d TPS (total)", totalPostRate)
	logToFile("GET Rate Limit: %d TPS (total)", totalGetRate)
	logToFile("CSV Balance Query Rate: %d QPS", csvRateLimit)
	logToFile("")

	// Multi-node efficiency analysis
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
	logToFile("Mint Throughput: %.2f mint operations/minute", float64(totalMintOperations)/(stressTestDuration.Minutes()))
	logToFile("Token Minting Efficiency: %.2f tokens/second", float64(totalTokensMinted)/stressTestDuration.Seconds())
	logToFile("Node Count: %d", len(nodeURLs))
	logToFile("Mint Operations per Node: %.2f ops/second/node", float64(totalMintOperations)/stressTestDuration.Seconds()/float64(len(nodeURLs)))
	logToFile("=== END OF TIMING STATISTICS ===")

	return nil
}

// RunStressTest executes the complete stress test workflow
func (st *StressTester) RunStressTest() error {
	log.Printf("Starting stress test: %d nodes, %d mint wallets, %d target wallets", 
		st.nodePool.Size(), MINT_WALLETS_COUNT, TRANSFER_WALLETS_COUNT)

	// Preparation: Create wallets
	if err := st.createMintWallets(); err != nil {
		return fmt.Errorf("failed to create mint wallets: %w", err)
	}
	log.Println("✓ Mint wallets created")

	if err := st.createTransferWallets(); err != nil {
		return fmt.Errorf("failed to create transfer wallets: %w", err)
	}
	log.Println("✓ Transfer wallets created")

	// Create token
	if err := st.createToken(); err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}
	log.Println("✓ Token created")

	// Phase 1: Grant all authorities first
	log.Println("Phase 1: Granting authorities...")
	if err := st.grantMintAuthorities(); err != nil {
		return fmt.Errorf("phase 1 failed: %w", err)
	}
	log.Println("✓ Phase 1: All authorities granted")

	// Phase 2: Perform minting operations
	if err := st.performConcurrentMinting(); err != nil {
		return fmt.Errorf("minting phase failed: %w", err)
	}
	log.Println("✓ All phases completed")

	log.Printf("✓ Test completed! Token: %s, Mints: %d",
		st.tokenAddress, MINT_WALLETS_COUNT*WALLETS_PER_MINT)

	return nil
}
