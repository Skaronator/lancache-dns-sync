package scheduler

import (
	"fmt"
	"time"
)

const DefaultSyncInterval = 24 * time.Hour

func ParseSyncInterval(intervalStr string) (time.Duration, error) {
	if intervalStr == "" {
		return DefaultSyncInterval, nil
	}

	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		return DefaultSyncInterval, fmt.Errorf("invalid sync interval '%s': %w", intervalStr, err)
	}

	// Validate minimum interval (prevent too frequent syncing)
	minInterval := 1 * time.Minute
	if duration < minInterval {
		return DefaultSyncInterval, fmt.Errorf("sync interval too short: %v (minimum: %v)", duration, minInterval)
	}

	return duration, nil
}