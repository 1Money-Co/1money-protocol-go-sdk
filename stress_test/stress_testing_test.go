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

	// Redirect default log output to file
	originalLogOutput := log.Writer()
	log.SetOutput(logFile)
	defer log.SetOutput(originalLogOutput)

	// Helper function to log to both test output and file
	logToFile := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fileLogger.Println(message)
		t.Log(message) // Still log to test output for immediate feedback
	}

	logToFile("Initializing 1Money Batch Mint Stress Tester...")
	logToFile("Log file created: %s", logFileName)

	// Run the complete stress test using shared function (legacy single node)
	if err := runCompleteStressTestLegacy(logToFile, fileLogger); err != nil {
		t.Fatal(err)
	}

	// Final log message with file location
	logToFile("All logs have been written to: %s", logFileName)
	t.Logf("Complete multi-tier stress test logs saved to: %s", logFileName)
}
