package scheduler_test

import (
	"testing"
	"time"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/scheduler"
	"github.com/stretchr/testify/require"
)

func TestComputeBackoff_Bounds(t *testing.T) {
	cfg := &config.Config{
		BaseDelaySeconds:  1,
		MaxDelaySeconds:   60,
		GlobalMaxAttempts: 30,
	}

	base := 1 * time.Second
	maxDelay := 60 * time.Second

	// Verify for attempts 0 to 10 that delay is always in [0, min(max, base * 2^attempts)]
	for attempts := 0; attempts <= 10; attempts++ {
		capDelay := base * (1 << attempts)
		if capDelay > maxDelay {
			capDelay = maxDelay
		}

		for i := 0; i < 50; i++ {
			delay := scheduler.ComputeBackoff(attempts, 0, 0, cfg)
			require.GreaterOrEqual(t, delay, time.Duration(0), "delay must never be negative")
			require.LessOrEqual(t, delay, capDelay, "delay must not exceed exponential ceiling")
		}
	}
}

func TestComputeBackoff_MaxDelayCapped(t *testing.T) {
	cfg := &config.Config{
		BaseDelaySeconds:  1,
		MaxDelaySeconds:   30,
		GlobalMaxAttempts: 30,
	}

	maxDelay := 30 * time.Second

	// Test extreme attempts that would overflow integer if uncapped
	extremeAttempts := []int{15, 30, 50, 64, 100}
	for _, attempts := range extremeAttempts {
		for i := 0; i < 20; i++ {
			delay := scheduler.ComputeBackoff(attempts, 0, 0, cfg)
			require.GreaterOrEqual(t, delay, time.Duration(0))
			require.LessOrEqual(t, delay, maxDelay, "delay must never exceed MaxDelaySeconds")
		}
	}
}

func TestComputeBackoff_DistributionSpread(t *testing.T) {
	cfg := &config.Config{
		BaseDelaySeconds:  5,
		MaxDelaySeconds:   60,
		GlobalMaxAttempts: 30,
	}

	// 100 runs on attempt 3 (ceiling = 5s * 8 = 40s)
	uniqueValues := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		delay := scheduler.ComputeBackoff(3, 0, 0, cfg)
		uniqueValues[delay] = true
	}

	// Full jitter must produce high variety of distinct timestamps
	require.Greater(t, len(uniqueValues), 50, "Full Jitter must produce a wide spread of random delays")
}

func TestComputeBackoff_SafeDefaults(t *testing.T) {
	// Nil config and negative attempts
	delay := scheduler.ComputeBackoff(-5, 0, 0, nil)
	require.GreaterOrEqual(t, delay, time.Duration(0))
	require.LessOrEqual(t, delay, 1*time.Second, "negative attempts should default to attempt 0 with default 1s base")

	// Custom base and max overrides
	customBase := 200 * time.Millisecond
	customMax := 800 * time.Millisecond
	delayCustom := scheduler.ComputeBackoff(2, customBase, customMax, nil)
	require.GreaterOrEqual(t, delayCustom, time.Duration(0))
	require.LessOrEqual(t, delayCustom, customMax)
}
