package scheduler

import (
	"testing"
	"time"
)

func TestParseSyncInterval(t *testing.T) {
	tests := []struct {
		name        string
		intervalStr string
		expectedDur time.Duration
		expectError bool
	}{
		// Valid duration formats
		{"1 hour", "1h", time.Hour, false},
		{"30 minutes", "30m", 30 * time.Minute, false},
		{"2 hours 30 minutes", "2h30m", 2*time.Hour + 30*time.Minute, false},
		{"24 hours", "24h", 24 * time.Hour, false},
		{"1 day", "24h", 24 * time.Hour, false},
		{"1 minute minimum", "1m", time.Minute, false},

		// Empty string should return default
		{"empty string (default)", "", DefaultSyncInterval, false},

		// Invalid formats
		{"invalid string", "invalid", DefaultSyncInterval, true},
		{"negative duration", "-1h", DefaultSyncInterval, true},
		{"too short interval", "30s", DefaultSyncInterval, true},
		{"zero duration", "0", DefaultSyncInterval, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dur, err := ParseSyncInterval(tt.intervalStr)

			if (err != nil) != tt.expectError {
				t.Errorf("ParseSyncInterval(%s) error = %v, expectError %v", tt.intervalStr, err, tt.expectError)
				return
			}

			if dur != tt.expectedDur {
				t.Errorf("ParseSyncInterval(%s) = %v, want %v", tt.intervalStr, dur, tt.expectedDur)
			}
		})
	}
}

func TestDefaultSyncInterval(t *testing.T) {
	if DefaultSyncInterval != 24*time.Hour {
		t.Errorf("DefaultSyncInterval = %v, want %v", DefaultSyncInterval, 24*time.Hour)
	}
}
