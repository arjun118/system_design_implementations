package config

import "time"

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
}

func Load() *Config {
	return &Config{
		DB: DBConfig{
			DatabaseURL: getEnv("DATABASE_URL", "postgres://jobq:durable@postgres:5432/jobq?sslmode=disable"),
			MaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 25),
			MaxIdleTime: getEnv("DB_MAX_IDLE_TIME", "15m"),
		},
		MinLeaseDurationSeconds: 5,
		MaxLeaseDurationSeconds: int(10 * time.Minute),
	}
}
