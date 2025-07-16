package main

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

func TestBatchMint(t *testing.T) {
	// Create log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("stress_test_%s.log", timestamp)
	logFile, err := os.Create(logFileName)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
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

	// Helper function to log to both test output and file
	logToFile := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fileLogger.Println(message)
		t.Log(message) // Still log to test output for immediate feedback
	}

	logToFile("Initializing 1Money Batch Mint Stress Tester...")
	logToFile("Log file created: %s", logFileName)

	// Run the complete stress test using default testnet node
	defaultNodeURL := "https://testapi.1moneynetwork.com"
	nodeURLs := []string{defaultNodeURL}
	
	if err := runCompleteStressTest(logToFile, fileLogger, nodeURLs, POST_RATE_LIMIT_TPS, GET_RATE_LIMIT_TPS); err != nil {
		t.Fatal(err)
	}

	// Final log message with file location
	logToFile("All logs have been written to: %s", logFileName)
	t.Logf("Complete multi-tier stress test logs saved to: %s", logFileName)
}
