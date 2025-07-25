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

// New creates a new URL exporter server
func New(cfg *config.Config) (*URLExporterServer, error) {
	// Create checker and collector
	chk := checker.New(cfg)
	col := metrics.NewCollector(cfg, chk)

	// Register collector with Prometheus
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

// setupRoutes configures the HTTP routes using jasoet/pkg/server patterns
func (s *URLExporterServer) setupRoutes(e *echo.Echo) {
	// Routes
	e.GET("/", s.handleRoot)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}

// handleRoot handles the root endpoint
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

// startBackgroundWorkers starts the checker and collector
func (s *URLExporterServer) startBackgroundWorkers(ctx context.Context) {
	// Start checker
	go s.checker.Start(ctx)
	// Start collector to process results
	go s.collector.Start(ctx)
}

// Start starts the HTTP server using jasoet/pkg/server patterns
func (s *URLExporterServer) Start() error {
	log.Info().Int("port", s.config.ListenPort).Msg("Starting URL Exporter server")
	
	// Use jasoet/pkg/server.Start function
	server.Start(
		s.config.ListenPort,
		func(e *echo.Echo) {
			// Setup routes
			s.setupRoutes(e)
			
			// Start background workers
			ctx := context.Background()
			s.startBackgroundWorkers(ctx)
			
			log.Info().Msg("URL Exporter server started successfully")
		},
		func(e *echo.Echo) {
			// Cleanup on shutdown
			log.Info().Msg("Shutting down URL Exporter server")
			
			// Shutdown checker
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

