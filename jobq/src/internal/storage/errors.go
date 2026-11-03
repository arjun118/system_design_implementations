package storage

import "errors"

var (
	ErrJobNotFound     = errors.New("job not found")
	ErrNoJobAvailable  = errors.New("no job available")
	ErrQueueNotFound   = errors.New("queue not found")
	ErrStaleLeaseToken = errors.New("state mismatch, stale lease token")
	ErrJobNotDead      = errors.New("job is not in dead status")
)
