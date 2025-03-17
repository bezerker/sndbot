package util

import (
	"log"
	"os"
)

// Logger is the global logger instance
var logger = log.New(os.Stdout, "", log.LstdFlags) // Changed to unexported

// IsDebugEnabled returns true if the DEBUG environment variable is set to "1" or "true"
func IsDebugEnabled() bool {
	debug := os.Getenv("DEBUG")
	return debug == "1" || debug == "true"
}

func CheckNilErr(e error) {
	if e != nil {
		Logger.Fatal(e)
	}
}
