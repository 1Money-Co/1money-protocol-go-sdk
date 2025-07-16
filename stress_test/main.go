package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

// main function for standalone execution
func main() {
	// Parse command line flags
	flag.Parse()

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("stress_test_%s.log", timestamp)
	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Create custom logger that writes to file
	fileLogger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	// Redirect default log output to file with microseconds
	originalLogOutput := log.Writer()
	originalLogFlags := log.Flags()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	defer func() {
		log.SetOutput(originalLogOutput)
		log.SetFlags(originalLogFlags)
	}()

	// Helper function to log only to file (same behavior as test method)
	logToFile := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fileLogger.Println(message)
		// No console output - logs only go to file like in test method
	}

	// Node list is required for multi-node stress testing
	if *nodeList == "" {
		errorMsg := "Node list is required. Use -nodes flag to specify comma-separated node URLs"
		fileLogger.Println("FATAL: " + errorMsg)
		fmt.Printf("FATAL: %s\n", errorMsg)
		fmt.Printf("Example: ./stress_test -nodes=127.0.0.1:18555,127.0.0.1:18556\n")
		os.Exit(1)
	}

	logToFile("Initializing 1Money Multi-Node Batch Mint Stress Tester...")
	logToFile("Log file created: %s", logFileName)

	// Parse node URLs
	nodeURLs, err := ParseNodeURLs(*nodeList)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to parse node list: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		fmt.Printf("FATAL: %s\n", errorMsg)
		os.Exit(1)
	}

	// Run the stress test
	if err := runCompleteStressTest(logToFile, fileLogger, nodeURLs, *postRate, *getRate); err != nil {
		errorMsg := fmt.Sprintf("Batch mint stress test failed: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		fmt.Printf("FATAL: Stress test failed. Check log file for details: %s\n", logFileName)
		os.Exit(1)
	}

	fmt.Printf("✓ Stress test completed successfully!\n")

	// Final log message with file location (only to file)
	logToFile("All logs have been written to: %s", logFileName)
	// Show completion message on console with log file location
	fmt.Printf("Logs: %s\n", logFileName)
}
