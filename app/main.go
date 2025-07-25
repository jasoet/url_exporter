package main

import (
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/jasoet/url-exporter/internal/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

func main() {
	// Setup logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warn().Str("level", cfg.LogLevel).Msg("Invalid log level, using info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Info().
		Str("instance", cfg.InstanceID).
		Int("port", cfg.ListenPort).
		Int("targets", len(cfg.Targets)).
		Str("check_interval", cfg.CheckInterval.String()).
		Str("timeout", cfg.Timeout.String()).
		Msg("Starting URL Exporter")

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create server")
	}

	// Start server (this will block and handle shutdown automatically via jasoet/pkg/server)
	if err := srv.Start(); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
