package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jasoet/pkg/server"
	"github.com/jasoet/url-exporter/internal/checker"
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/jasoet/url-exporter/internal/metrics"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// URLExporterServer holds the application components
type URLExporterServer struct {
	config    *config.Config
	checker   *checker.Checker
	collector *metrics.Collector
}

func New(cfg *config.Config) (*URLExporterServer, error) {
	chk := checker.New(cfg)
	col := metrics.NewCollector(cfg, chk)

	if err := col.Register(); err != nil {
		return nil, fmt.Errorf("failed to register metrics collector: %w", err)
	}

	s := &URLExporterServer{
		config:    cfg,
		checker:   chk,
		collector: col,
	}

	return s, nil
}

func (s *URLExporterServer) setupRoutes(e *echo.Echo) {
	e.GET("/", s.handleRoot)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}

func (s *URLExporterServer) handleRoot(c echo.Context) error {
	info := map[string]interface{}{
		"service":   "url-exporter",
		"version":   "1.0.0",
		"instance":  s.config.InstanceID,
		"targets":   len(s.config.Targets),
		"status":    "running",
		"endpoints": []string{"/", "/health", "/metrics"},
	}
	return c.JSON(http.StatusOK, info)
}

func (s *URLExporterServer) startBackgroundWorkers(ctx context.Context) {
	go s.checker.Start(ctx)
	go s.collector.Start(ctx)
}

func (s *URLExporterServer) Start() error {
	log.Info().Int("port", s.config.ListenPort).Msg("Starting URL Exporter server")

	server.Start(
		s.config.ListenPort,
		func(e *echo.Echo) {
			s.setupRoutes(e)

			ctx := context.Background()
			s.startBackgroundWorkers(ctx)

			log.Info().Msg("URL Exporter server started successfully")
		},
		func(e *echo.Echo) {
			log.Info().Msg("Shutting down URL Exporter server")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := s.checker.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to shutdown checker")
			}

			log.Info().Msg("URL Exporter server shutdown complete")
		},
	)

	return nil
}
