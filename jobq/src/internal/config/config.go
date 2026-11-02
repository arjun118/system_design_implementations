package config

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
}

func Load() *Config {
	return &Config{
		DB: DBConfig{
			DatabaseURL: getEnv("DATABASE_URL", "postgres://jobq:durable@postgres:5432/jobq?sslmode=disable"),
			MaxOpenConn: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConn: getEnvInt("DB_MAX_IDLE_CONNS", 25),
			MaxIdleTime: getEnv("DB_MAX_IDLE_TIME", "15m"),
		},
		MinLeaseDurationSeconds: getEnvInt("MIN_LEASE_DURATION", 5),
		MaxLeaseDurationSeconds: getEnvInt("MAX_LEASE_DURATION", 10*60),
		ReaperPollDuration:      getEnvInt("REAPER_POLL_DURATION", 5),
		LeaseDurationIncrement:  getEnvInt("LEASE_DURATION_INCREMENT", 30),
	}
}
