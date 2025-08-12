package config

import (
	"os"
	"strconv"
)

// RecoveryConfig holds configuration for the recovery mechanism
type RecoveryConfig struct {
	// EnableRecovery globally enables/disables the recovery mechanism
	EnableRecovery bool
	
	// DefaultRecoveryEnabled sets the default value for new jobs
	DefaultRecoveryEnabled bool
	
	// RecoveryCheckInterval is how often to check for incomplete intervals (in seconds)
	RecoveryCheckInterval int
	
	// MaxRecoveryAttempts limits how many times a job can be recovered
	MaxRecoveryAttempts int
}

// LoadRecoveryConfig loads recovery configuration from environment variables
func LoadRecoveryConfig() *RecoveryConfig {
	config := &RecoveryConfig{
		EnableRecovery:        getEnvBool("ENABLE_RECOVERY", true),
		DefaultRecoveryEnabled: getEnvBool("DEFAULT_RECOVERY_ENABLED", true),
		RecoveryCheckInterval:  getEnvInt("RECOVERY_CHECK_INTERVAL", 300), // 5 minutes
		MaxRecoveryAttempts:    getEnvInt("MAX_RECOVERY_ATTEMPTS", 3),
	}
	
	return config
}

// getEnvBool gets a boolean environment variable with a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}