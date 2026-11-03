package config

import (
	"github.com/rs/zerolog"
)

type DBConfig struct {
	DatabaseURL string
	MaxOpenConn int
	MaxIdleConn int
	MaxIdleTime string
}

type Config struct {
	DB                      DBConfig
	MinLeaseDurationSeconds int
	MaxLeaseDurationSeconds int
	ReaperPollDuration      int
	LeaseDurationIncrement  int
	BaseDelaySeconds        int
	MaxDelaySeconds         int
	GlobalMaxAttempts       int
}

func Load(logger zerolog.Logger) *Config {
	globalMaxAttempts := getEnvInt("GLOBAL_MAX_ATTEMPTS", 30)
	if globalMaxAttempts <= 0 || globalMaxAttempts > 30 {
		logger.Warn().Int("global_max_attempts", globalMaxAttempts).Msg("global_max_attempts clipped to 30")
		globalMaxAttempts = 30
	}

	minLeaseDurationSeconds := getEnvInt("MIN_LEASE_DURATION", 5)
	if minLeaseDurationSeconds <= 0 {
		logger.Warn().Int("min_lease_duration", minLeaseDurationSeconds).Msg("min_lease_duration must be positive, defaulting to 5s")
		minLeaseDurationSeconds = 5
	}

	maxLeaseDurationSeconds := getEnvInt("MAX_LEASE_DURATION", 10*60)
	if maxLeaseDurationSeconds <= minLeaseDurationSeconds {
		maxLeaseDurationSeconds = minLeaseDurationSeconds * 3
		maxLeaseDurationSeconds = max(maxLeaseDurationSeconds, 600)
		logger.Warn().
			Int("min_lease_duration", minLeaseDurationSeconds).
			Int("adjusted_max_lease_duration", maxLeaseDurationSeconds).
			Msg("max_lease_duration must exceed min_lease_duration, auto-adjusted to at least min * 3")
	}

	reaperPollDuration := getEnvInt("REAPER_POLL_DURATION", 5)
	if reaperPollDuration <= 0 {
		logger.Warn().Int("reaper_poll_duration", reaperPollDuration).Msg("reaper_poll_duration must be positive, defaulting to 5s")
		reaperPollDuration = 5
	}

	leaseDurationIncrement := getEnvInt("LEASE_DURATION_INCREMENT", 30)
	if leaseDurationIncrement <= 0 {
		logger.Warn().Int("lease_duration_increment", leaseDurationIncrement).Msg("lease_duration_increment must be positive, defaulting to 30s")
		leaseDurationIncrement = 30
	}

	baseDelaySeconds := getEnvInt("BASE_DELAY_SECONDS", 1)
	if baseDelaySeconds <= 0 {
		logger.Warn().Int("base_delay_seconds", baseDelaySeconds).Msg("base_delay_seconds must be positive, defaulting to 1s")
		baseDelaySeconds = 1
	}

	maxDelaySeconds := getEnvInt("MAX_DELAY_SECONDS", 60)
	if maxDelaySeconds <= baseDelaySeconds {
		maxDelaySeconds = baseDelaySeconds * 10
		maxDelaySeconds = max(maxDelaySeconds, 60)
		logger.Warn().
			Int("base_delay_seconds", baseDelaySeconds).
			Int("adjusted_max_delay_seconds", maxDelaySeconds).
			Msg("max_delay_seconds must exceed base_delay_seconds, auto-adjusted")
	}

	return &Config{
		DB: DBConfig{
			DatabaseURL: getEnv("DATABASE_URL", "postgres://jobq:durable@postgres:5432/jobq?sslmode=disable"),
			MaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 25),
			MaxIdleTime: getEnv("DB_MAX_IDLE_TIME", "15m"),
		},
		MinLeaseDurationSeconds: minLeaseDurationSeconds,
		MaxLeaseDurationSeconds: maxLeaseDurationSeconds,
		ReaperPollDuration:      reaperPollDuration,
		LeaseDurationIncrement:  leaseDurationIncrement,
		BaseDelaySeconds:        baseDelaySeconds,
		MaxDelaySeconds:         maxDelaySeconds,
		GlobalMaxAttempts:       globalMaxAttempts,
	}
}
