package storage

import (
	"fmt"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/infra"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool   *pgxpool.Pool
	config *config.Config
}

func NewStore(cfg *config.Config) (*Store, error) {
	pool, err := infra.NewPgPool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate db pool, %w", err)
	}
	return &Store{
		pool:   pool,
		config: cfg,
	}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}
