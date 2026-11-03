package scheduler

import (
	"math/rand/v2"
	"time"

	"arjun118.github.io/jobq/internal/config"
)

func ComputeBackoff(attempts int, baseDelay, maxDelay time.Duration, cfg *config.Config) time.Duration {
	if baseDelay <= 0 && cfg != nil {
		baseDelay = time.Duration(cfg.BaseDelaySeconds) * time.Second
	}
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	if maxDelay <= 0 && cfg != nil {
		maxDelay = time.Duration(cfg.MaxDelaySeconds) * time.Second
	}
	if maxDelay <= 0 {
		maxDelay = 60 * time.Second
	}
	if attempts < 0 {
		attempts = 0
	}
	maxAttempts := 30
	if cfg != nil && cfg.GlobalMaxAttempts > 0 {
		maxAttempts = cfg.GlobalMaxAttempts
	}
	if attempts > maxAttempts {
		attempts = maxAttempts
	}
	delay := baseDelay * (1 << attempts)
	if delay > maxDelay || delay <= 0 {
		delay = maxDelay
	}
	return time.Duration(rand.Int64N(int64(delay) + 1))
}
