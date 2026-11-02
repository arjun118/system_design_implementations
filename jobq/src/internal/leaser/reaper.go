package leaser

import (
	"context"
	"time"

	"arjun118.github.io/jobq/internal/storage"
	"github.com/rs/zerolog"
)

type Reaper struct {
	store    *storage.Store
	interval time.Duration
	logger   zerolog.Logger
}

func NewReaper(store *storage.Store, interval time.Duration, logger zerolog.Logger) *Reaper {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Reaper{
		store:    store,
		interval: interval,
		logger:   logger,
	}
}

func (r *Reaper) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	r.logger.Info().Dur("interval", r.interval).Msg("lease reaper started")
	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("lease reaper stopping")
			return
		case <-ticker.C:
			reclaimed, err := r.store.ReapExpiredLeases(ctx)
			if err != nil {
				r.logger.Error().Err(err).Msg("error reaping expired leases")
				continue
			}
			if reclaimed > 0 {
				r.logger.Info().Int64("reclaimed_jobs", reclaimed).Msg("reclaimed expired job leases")
			}
		}
	}

}
