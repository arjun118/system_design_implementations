package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/httpapi"
	"arjun118.github.io/jobq/internal/leaser"
	"arjun118.github.io/jobq/internal/logger"
	"arjun118.github.io/jobq/internal/storage"
)

func main() {
	appLogger := logger.New(os.Stdout)
	cfg := config.Load(appLogger)
	pgPool, err := storage.NewStore(cfg)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("failed to initiate db pool")
	}
	reaperLogger := appLogger.With().Str("component", "reaper").Logger()
	reaper := leaser.NewReaper(pgPool, time.Duration(cfg.ReaperPollDuration)*time.Second, reaperLogger)
	go reaper.Start(context.Background())
	server := httpapi.NewServer(appLogger, pgPool)
	fmt.Println("starting server...")
	appLogger.Info().Msg("starting jobq server on :8080")
	if err := http.ListenAndServe(":8080", server); err != nil {
		appLogger.Error().Err(err).Msg("server fatal")
		os.Exit(1)
	}
}
