package checker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jasoet/pkg/concurrent"
	"github.com/jasoet/pkg/rest"
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/rs/zerolog/log"
)

// Result represents the result of a URL check
type Result struct {
	URL          string
	Host         string
	Path         string
	StatusCode   int
	ResponseTime time.Duration
	Error        error
	Timestamp    time.Time
}

// Checker performs URL availability checks
type Checker struct {
	config     *config.Config
	restClient *rest.Client
	results    chan Result
	cancel     context.CancelFunc
	mutex      sync.RWMutex
}

func New(cfg *config.Config) *Checker {
	restConfig := &rest.Config{
		RetryCount:    cfg.Retries,
		RetryWaitTime: time.Second,
		Timeout:       cfg.Timeout,
	}

	restClient := rest.NewClient(rest.WithRestConfig(*restConfig))

	return &Checker{
		config:     cfg,
		restClient: restClient,
		results:    make(chan Result, len(cfg.Targets)*2),
	}
}

func (c *Checker) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.mutex.Lock()
	c.cancel = cancel
	c.mutex.Unlock()

	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	c.checkAllURLs(ctx)

	for {
		select {
		case <-ctx.Done():
			close(c.results)
			return
		case <-ticker.C:
			c.checkAllURLs(ctx)
		}
	}
}

func (c *Checker) Results() <-chan Result {
	return c.results
}

func (c *Checker) checkAllURLs(ctx context.Context) {
	funcs := make(map[string]concurrent.Func[Result])

	for i, targetURL := range c.config.Targets {
		funcKey := fmt.Sprintf("url_%d", i)
		targetURL := targetURL

		funcs[funcKey] = func(ctx context.Context) (Result, error) {
			result := c.checkURL(ctx, targetURL)
			if result.Error != nil {
				return result, nil
			}
			return result, nil
		}
	}

	results, err := concurrent.ExecuteConcurrently(ctx, funcs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute concurrent URL checks")
		return
	}

	for _, result := range results {
		select {
		case c.results <- result:
		case <-ctx.Done():
			return
		}
	}
}

func (c *Checker) checkURL(ctx context.Context, targetURL string) Result {
	host, path := parseURL(targetURL)

	result := Result{
		URL:       targetURL,
		Host:      host,
		Path:      path,
		Timestamp: time.Now(),
	}

	start := time.Now()
	statusCode, err := c.performCheck(ctx, targetURL)
	elapsed := time.Since(start)

	if err == nil {
		result.StatusCode = statusCode
		result.ResponseTime = elapsed
		result.Error = nil

		log.Debug().
			Str("url", targetURL).
			Int("status_code", statusCode).
			Dur("response_time", elapsed).
			Msg("URL check successful")

		return result
	}

	result.Error = err
	result.StatusCode = 0

	log.Error().
		Str("url", targetURL).
		Err(err).
		Msg("URL check failed")

	return result
}

func (c *Checker) performCheck(ctx context.Context, targetURL string) (int, error) {
	headers := map[string]string{
		"User-Agent": "url-exporter/1.0",
	}

	response, err := c.restClient.MakeRequest(ctx, http.MethodHead, targetURL, "", headers)
	if err != nil {
		var executionErr *rest.ExecutionError
		var unauthorizedErr *rest.UnauthorizedError
		var notFoundErr *rest.ResourceNotFoundError
		var serverErr *rest.ServerError
		var responseErr *rest.ResponseError

		switch {
		case errors.As(err, &executionErr):
			return 0, fmt.Errorf("network error: %w", executionErr)
		case errors.As(err, &unauthorizedErr):
			return unauthorizedErr.StatusCode, nil
		case errors.As(err, &notFoundErr):
			return notFoundErr.StatusCode, nil
		case errors.As(err, &serverErr):
			return serverErr.StatusCode, nil
		case errors.As(err, &responseErr):
			return responseErr.StatusCode, nil
		default:
			return 0, fmt.Errorf("request failed: %w", err)
		}
	}

	return response.StatusCode(), nil
}

func parseURL(targetURL string) (host, path string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return targetURL, "/"
	}

	host = u.Scheme + "://" + u.Host

	path = u.Path
	if path == "" {
		path = "/"
	}

	if u.RawQuery != "" {
		path = path + "?" + u.RawQuery
	}

	return host, path
}

func (c *Checker) Shutdown(_ context.Context) error {
	c.mutex.RLock()
	cancel := c.cancel
	c.mutex.RUnlock()

	if cancel != nil {
		cancel()
	}
	return nil
}
