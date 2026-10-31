package infra

import (
	"context"
	"fmt"
	"time"

	"arjun118.github.io/jobq/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPgPool(cfg *config.Config) (*pgxpool.Pool, error) {
	ctx := context.Background()

	poolConfig, err := pgxpool.ParseConfig(cfg.DB.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database url: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.DB.MaxOpenConn)
	poolConfig.MinConns = int32(cfg.DB.MaxIdleConn)
	maxIdleDuration, err := time.ParseDuration(cfg.DB.MaxIdleTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max idle time: %w", err)
	}
	poolConfig.MaxConnIdleTime = maxIdleDuration

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize db pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = pool.Ping(pingCtx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}
	return pool, nil

}
