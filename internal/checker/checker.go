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

// New creates a new URL checker  
func New(cfg *config.Config) *Checker {
	// Create HTTP client with timeout and skip SSL verification for internal services
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Set to true if needed for internal services
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	return &Checker{
		config:     cfg,
		httpClient: httpClient,
		results:    make(chan Result, len(cfg.Targets)*2), // Buffer for results
	}
}

// Start begins checking URLs periodically
func (c *Checker) Start(ctx context.Context) {
	// Create context with cancel for this checker
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	// Initial check
	c.checkAllURLs(ctx)

	// Periodic checks
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

// Results returns the results channel
func (c *Checker) Results() <-chan Result {
	return c.results
}

// checkAllURLs checks all configured URLs concurrently using jasoet/pkg/concurrent
func (c *Checker) checkAllURLs(ctx context.Context) {
	// Create functions map for concurrent execution
	funcs := make(map[string]concurrent.Func[Result])
	
	for i, targetURL := range c.config.Targets {
		funcKey := fmt.Sprintf("url_%d", i)
		targetURL := targetURL // Capture for closure
		
		funcs[funcKey] = func(ctx context.Context) (Result, error) {
			result := c.checkURL(ctx, targetURL)
			// Convert result to function return format
			if result.Error != nil {
				// Still return the result with error for processing
				return result, nil
			}
			return result, nil
		}
	}
	
	// Execute all URL checks concurrently
	results, err := concurrent.ExecuteConcurrently(ctx, funcs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute concurrent URL checks")
		return
	}
	
	// Send results to channel
	for _, result := range results {
		select {
		case c.results <- result:
		case <-ctx.Done():
			return
		}
	}
}

// checkURL checks a single URL with retry logic
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
			// Wait before retry (exponential backoff)
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

	// All retries failed
	result.Error = lastErr
	result.StatusCode = 0
	
	log.Error().
		Str("url", targetURL).
		Err(lastErr).
		Msg("URL check failed after all retries")
	
	return result
}

// performCheck performs the actual HTTP request
func (c *Checker) performCheck(ctx context.Context, targetURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "url-exporter/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// parseURL extracts host and path from a URL
func parseURL(targetURL string) (host, path string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return targetURL, "/"
	}

	// Build host with scheme
	host = u.Scheme + "://" + u.Host

	// Get path, default to "/" if empty
	path = u.Path
	if path == "" {
		path = "/"
	}

	// Include query string if present
	if u.RawQuery != "" {
		path = path + "?" + u.RawQuery
	}

	return host, path
}

// Shutdown gracefully shuts down the checker
func (c *Checker) Shutdown(ctx context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}