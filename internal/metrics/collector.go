package metrics

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/jasoet/url-exporter/internal/checker"
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

// Collector implements the Prometheus collector interface
type Collector struct {
	config      *config.Config
	checker     *checker.Checker
	mutex       sync.RWMutex
	lastResults map[string]*checker.Result

	// Metrics
	urlUp              *prometheus.Desc
	urlResponseTime    *prometheus.Desc
	urlHTTPStatusCode  *prometheus.Desc
	urlCheckTotal      *prometheus.Desc
	urlStatusCodeTotal *prometheus.Desc
}

// NewCollector creates a new metrics collector
func NewCollector(cfg *config.Config, chk *checker.Checker) *Collector {
	return &Collector{
		config:      cfg,
		checker:     chk,
		lastResults: make(map[string]*checker.Result),

		urlUp: prometheus.NewDesc(
			"url_up",
			"URL is up (1 if URL returns 2xx status, 0 otherwise)",
			[]string{"url", "host", "path", "instance"},
			nil,
		),
		urlResponseTime: prometheus.NewDesc(
			"url_response_time_milliseconds",
			"Response time in milliseconds",
			[]string{"url", "host", "path", "instance"},
			nil,
		),
		urlHTTPStatusCode: prometheus.NewDesc(
			"url_http_status_code",
			"HTTP status code returned",
			[]string{"url", "host", "path", "instance"},
			nil,
		),
		urlCheckTotal: prometheus.NewDesc(
			"url_check_total",
			"Total number of checks by status code",
			[]string{"url", "host", "path", "status_code", "instance"},
			nil,
		),
		urlStatusCodeTotal: prometheus.NewDesc(
			"url_status_code_total",
			"Counter for each specific HTTP status code encountered",
			[]string{"url", "host", "path", "status_code", "instance"},
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.urlUp
	ch <- c.urlResponseTime
	ch <- c.urlHTTPStatusCode
	ch <- c.urlCheckTotal
	ch <- c.urlStatusCodeTotal
}

// Collect implements prometheus.Collector
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Create metrics for each URL result
	for _, result := range c.lastResults {
		labels := []string{result.URL, result.Host, result.Path, c.config.InstanceID}

		// Determine if URL is up (2xx status code)
		up := float64(0)
		if result.Error == nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			up = 1
		}

		// url_up metric
		ch <- prometheus.MustNewConstMetric(
			c.urlUp,
			prometheus.GaugeValue,
			up,
			labels...,
		)

		// url_response_time_milliseconds metric (only if successful)
		if result.Error == nil {
			ch <- prometheus.MustNewConstMetric(
				c.urlResponseTime,
				prometheus.GaugeValue,
				float64(result.ResponseTime.Milliseconds()),
				labels...,
			)

			// url_http_status_code metric
			ch <- prometheus.MustNewConstMetric(
				c.urlHTTPStatusCode,
				prometheus.GaugeValue,
				float64(result.StatusCode),
				labels...,
			)
		}
	}
}

// Start starts processing checker results
func (c *Collector) Start(ctx context.Context) {
	// Initialize counters for all URLs
	counters := make(map[string]map[string]int)
	for _, url := range c.config.Targets {
		counters[url] = make(map[string]int)
	}

	// Process results from checker
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-c.checker.Results():
			if !ok {
				return
			}

			// Update last result
			c.mutex.Lock()
			c.lastResults[result.URL] = &result
			c.mutex.Unlock()

			// Update counters
			statusCode := "error"
			if result.Error == nil {
				statusCode = strconv.Itoa(result.StatusCode)
			}

			if _, exists := counters[result.URL]; !exists {
				counters[result.URL] = make(map[string]int)
			}
			counters[result.URL][statusCode]++

			log.Debug().
				Str("url", result.URL).
				Str("status", statusCode).
				Msg("Processed check result")
		}
	}
}

// Register registers the collector with Prometheus
func (c *Collector) Register() error {
	if err := prometheus.Register(c); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}
	return nil
}
