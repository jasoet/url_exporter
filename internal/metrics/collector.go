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
	counters    map[string]map[string]int // URL -> status_code -> count

	urlUp              *prometheus.Desc
	urlError           *prometheus.Desc
	urlResponseTime    *prometheus.Desc
	urlHTTPStatusCode  *prometheus.Desc
	urlCheckTotal      *prometheus.Desc
	urlStatusCodeTotal *prometheus.Desc
}

func NewCollector(cfg *config.Config, chk *checker.Checker) *Collector {
	return &Collector{
		config:      cfg,
		checker:     chk,
		lastResults: make(map[string]*checker.Result),
		counters:    make(map[string]map[string]int),

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
		urlError: prometheus.NewDesc(
			"url_error",
			"URL error (1 if URL returns network/connection error, 0 otherwise)",
			[]string{"url", "host", "path", "instance"},
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

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.urlUp
	ch <- c.urlError
	ch <- c.urlResponseTime
	ch <- c.urlHTTPStatusCode
	ch <- c.urlCheckTotal
	ch <- c.urlStatusCodeTotal
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, result := range c.lastResults {
		labels := []string{result.URL, result.Host, result.Path, c.config.InstanceID}

		up := float64(0)
		if result.Error == nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			up = 1
		}

		ch <- prometheus.MustNewConstMetric(
			c.urlUp,
			prometheus.GaugeValue,
			up,
			labels...,
		)

		errorValue := float64(0)
		if result.Error != nil {
			errorValue = 1
		}

		ch <- prometheus.MustNewConstMetric(
			c.urlError,
			prometheus.GaugeValue,
			errorValue,
			labels...,
		)

		if result.Error == nil {
			ch <- prometheus.MustNewConstMetric(
				c.urlResponseTime,
				prometheus.GaugeValue,
				float64(result.ResponseTime.Milliseconds()),
				labels...,
			)

			ch <- prometheus.MustNewConstMetric(
				c.urlHTTPStatusCode,
				prometheus.GaugeValue,
				float64(result.StatusCode),
				labels...,
			)
		}
	}

	for url, statusCounts := range c.counters {
		result, exists := c.lastResults[url]
		if !exists {
			continue
		}

		baseLabels := []string{url, result.Host, result.Path}

		for statusCode, count := range statusCounts {
			checkLabels := append(baseLabels, statusCode, c.config.InstanceID)
			ch <- prometheus.MustNewConstMetric(
				c.urlCheckTotal,
				prometheus.CounterValue,
				float64(count),
				checkLabels...,
			)

			statusLabels := append(baseLabels, statusCode, c.config.InstanceID)
			ch <- prometheus.MustNewConstMetric(
				c.urlStatusCodeTotal,
				prometheus.CounterValue,
				float64(count),
				statusLabels...,
			)
		}
	}
}

func (c *Collector) Start(ctx context.Context) {
	c.mutex.Lock()
	for _, url := range c.config.Targets {
		c.counters[url] = make(map[string]int)
	}
	c.mutex.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-c.checker.Results():
			if !ok {
				return
			}

			c.mutex.Lock()
			c.lastResults[result.URL] = &result

			statusCode := "error"
			if result.Error == nil {
				statusCode = strconv.Itoa(result.StatusCode)
			}

			if _, exists := c.counters[result.URL]; !exists {
				c.counters[result.URL] = make(map[string]int)
			}
			c.counters[result.URL][statusCode]++
			c.mutex.Unlock()

			log.Debug().
				Str("url", result.URL).
				Str("status", statusCode).
				Msg("Processed check result")
		}
	}
}

func (c *Collector) Register() error {
	if err := prometheus.Register(c); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}
	return nil
}
