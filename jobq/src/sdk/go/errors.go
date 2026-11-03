package jobq

import "errors"

var (
	ErrNoJobAvailable = errors.New("no job available")
	ErrJobNotFound    = errors.New("no job found with that id")

	ErrStaleLeaseToken = errors.New("stale lease token")
	ErrJobNotDead      = errors.New("job is not in dead status")
)
