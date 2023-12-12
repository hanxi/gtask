package config

import (
	"testing"

	"github.com/hanxi/gtask/log"
)

func TestLoad(t *testing.T) {
	configPath := "config.json"
	Load(configPath)

	t.Log("configPath", configPath)
	t.Log("config.C", C)

	// Log a debug message
	log.Debug("This is a debug message")
	// Log an info message
	log.Info("This is an info message")
}
