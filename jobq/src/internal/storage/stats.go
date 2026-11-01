package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type QueueStats struct {
	Queue        string `json:"queue" db:"queue"`
	Pending      int64  `json:"pending" db:"pending"`
	Running      int64  `json:"running" db:"running"`
	Completed    int64  `json:"completed" db:"completed"`
	Failed       int64  `json:"failed" db:"failed"`
	DeadLettered int64  `json:"dead_lettered" db:"dead_lettered"`
	Total        int64  `json:"total" db:"total"`
}

func (s *Store) GetStats(ctx context.Context, queueName string) (*QueueStats, error) {
	query := `
		select $1 as queue,
			count(*) filter (where status='pending') as pending,
			count(*) filter (where status='running') as running,
		 	count(*) filter (where status='completed') as completed,
			count(*) filter (where status='failed') as failed,
			count(*) filter (where status='dead') as dead_lettered,
			count(*) as total
		from jobs
		where queue=$1
	`
	args := []any{queueName}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query queue stats: %w", err)
	}
	defer rows.Close()

	stat, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[QueueStats])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrQueueNotFound
		}
		return nil, fmt.Errorf("failed to scan queue stats: %w", err)
	}
	return stat, nil
}
