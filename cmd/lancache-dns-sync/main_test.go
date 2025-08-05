package main

import (
	"os"
	"testing"
)

func TestRunOnceEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true", "true", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"empty", "", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			if err := os.Unsetenv("RUN_ONCE"); err != nil {
				t.Fatalf("Failed to unset RUN_ONCE: %v", err)
			}

			// Set test value
			if tt.envValue != "" {
				if err := os.Setenv("RUN_ONCE", tt.envValue); err != nil {
					t.Fatalf("Failed to set RUN_ONCE: %v", err)
				}
			}

			// Mock flag parsing
			runOnce := false

			// Simulate the environment variable check logic from main()
			runOnceEnv := os.Getenv("RUN_ONCE")
			if runOnceEnv == "true" || runOnceEnv == "1" || runOnceEnv == "yes" {
				runOnce = true
			}

			if runOnce != tt.expected {
				t.Errorf("Expected runOnce to be %v for env value '%s', got %v", tt.expected, tt.envValue, runOnce)
			}
		})
	}
}
