package storage

import (
	"fmt"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/infra"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(cfg *config.Config) (*Store, error) {
	pool, err := infra.NewPgPool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate db pool, %w", err)
	}
	return &Store{
		pool: pool,
	}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
