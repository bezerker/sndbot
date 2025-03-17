package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	Logger  *log.Logger
	logFile *os.File
)

func InitLogger() error {
	// Create logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Open log file with append mode
	logFile, err = os.OpenFile(filepath.Join("logs", "sndbot.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Create multi-writer to write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Initialize logger with timestamp and caller info
	Logger = log.New(multiWriter, "", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}
