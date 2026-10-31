package storage

import "errors"

var (
	ErrJobNotFound   = errors.New("job not found")
	ErrQueueNotFound = errors.New("queue not found")
)
