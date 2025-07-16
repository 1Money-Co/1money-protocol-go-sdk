package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	file   *os.File
	logger *log.Logger
	mu     sync.Mutex
}

var globalLogger *Logger

func InitLogger() (*Logger, error) {
	timestamp := time.Now().Format("20060102_150405")
	logFilename := fmt.Sprintf("load_runner_%s.log", timestamp)
	
	logFile, err := os.Create(logFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}
	
	// Create logger with custom format including milliseconds
	globalLogger = &Logger{
		file:   logFile,
		logger: log.New(logFile, "", 0), // No default flags, we'll format manually
	}
	
	absPath, _ := filepath.Abs(logFilename)
	fmt.Printf("Log file created: %s\n", absPath)
	
	return globalLogger, nil
}

func (l *Logger) Printf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006/01/02 15:04:05.000")
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("%s %s", timestamp, msg)
}

func (l *Logger) Println(v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006/01/02 15:04:05.000")
	msg := fmt.Sprintln(v...)
	l.logger.Printf("%s %s", timestamp, msg)
}

func (l *Logger) Close() error {
	return l.file.Close()
}

func Logf(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Printf(format, v...)
	}
}

func Logln(v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Println(v...)
	}
}