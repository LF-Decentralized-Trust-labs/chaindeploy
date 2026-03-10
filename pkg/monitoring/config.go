package monitoring

import (
	"time"
)

// Config represents the configuration for the monitoring service
type Config struct {
	// DefaultCheckInterval is the default interval for checking nodes
	DefaultCheckInterval time.Duration
	// DefaultTimeout is the default timeout for node checks
	DefaultTimeout time.Duration
	// DefaultFailureThreshold is the default number of consecutive failures before alerting
	DefaultFailureThreshold int
	// Workers is the number of concurrent workers checking nodes
	Workers int
}

// DefaultConfig returns a Config with sensible default values
func DefaultConfig() *Config {
	return &Config{
		DefaultCheckInterval:    1 * time.Minute,
		DefaultTimeout:          2 * time.Second,
		DefaultFailureThreshold: 3,
		Workers:                 5,
	}
}
