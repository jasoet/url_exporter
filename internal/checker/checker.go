package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/jasoet/pkg/concurrent"
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
	httpClient *http.Client
	results    chan Result
	cancel     context.CancelFunc
}

func New(cfg *config.Config) *Checker {
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	return &Checker{
		config:     cfg,
		httpClient: httpClient,
		results:    make(chan Result, len(cfg.Targets)*2),
	}
}

func (c *Checker) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

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

	var lastErr error
	for attempt := 0; attempt <= c.config.Retries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
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

		lastErr = err
		log.Warn().
			Str("url", targetURL).
			Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", c.config.Retries+1).
			Msg("URL check failed")
	}

	result.Error = lastErr
	result.StatusCode = 0

	log.Error().
		Str("url", targetURL).
		Err(lastErr).
		Msg("URL check failed after all retries")

	return result
}

func (c *Checker) performCheck(ctx context.Context, targetURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "url-exporter/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
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
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
