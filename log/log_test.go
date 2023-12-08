package log

import (
	"testing"
)

func TestSetLevel(t *testing.T) {
	// Set the logging level to DEBUG
	SetLevel("DEBUG")

	// Log a debug message
	Debug("This is a debug message")

	// Set the logging level to INFO
	SetLevel("INFO")

	// Log a debug message
	Debug("This is a debug message")

	// Log an info message
	Info("This is an info message")
}
