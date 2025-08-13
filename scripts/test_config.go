package main

import (
	"time"
)

// TestConfig holds configuration for the task recovery test
type TestConfig struct {
	// Database settings
	DatabaseURL string

	// Test settings
	TestJobName     string
	TestPrompt      string
	TestAgentName   string
	TestAgentURL    string
	WaitTimeSeconds int

	// Cleanup settings
	AutoCleanup bool
}

// DefaultTestConfig returns the default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		TestJobName:     "Test Recovery Job",
		TestPrompt:      "Test recovery scenario - automated test",
		TestAgentName:   "test_agent",
		TestAgentURL:    "http://localhost:8080",
		WaitTimeSeconds: 2,
		AutoCleanup:     true,
	}
}

// GetWaitTime returns the wait time as duration
func (c *TestConfig) GetWaitTime() time.Duration {
	return time.Duration(c.WaitTimeSeconds) * time.Second
}