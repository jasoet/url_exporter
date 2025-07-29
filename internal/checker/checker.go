package checker

import (
	"context"
	"errors"
	"fmt"
	"net"
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

// ProtocolChecker defines the interface for checking different protocols
type ProtocolChecker interface {
	Check(ctx context.Context, target string) (statusCode int, err error)
	Protocol() string
}

// HTTPChecker handles HTTP/HTTPS protocol checks
type HTTPChecker struct {
	restClient *rest.Client
}

// TelnetChecker handles non-HTTP protocol checks using telnet
type TelnetChecker struct {
	timeout time.Duration
}

// Checker performs URL availability checks
type Checker struct {
	config      *config.Config
	restClient  *rest.Client
	results     chan Result
	cancel      context.CancelFunc
	mutex       sync.RWMutex
	checkers    map[string]ProtocolChecker
}

// NewHTTPChecker creates a new HTTP protocol checker
func NewHTTPChecker(restClient *rest.Client) *HTTPChecker {
	return &HTTPChecker{
		restClient: restClient,
	}
}

// Check performs HTTP/HTTPS health check
func (h *HTTPChecker) Check(ctx context.Context, target string) (int, error) {
	headers := map[string]string{
		"User-Agent": "url-exporter/1.0",
	}

	response, err := h.restClient.MakeRequest(ctx, http.MethodHead, target, "", headers)
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

// Protocol returns the protocol name
func (h *HTTPChecker) Protocol() string {
	return "http"
}

// NewTelnetChecker creates a new telnet-based protocol checker
func NewTelnetChecker(timeout time.Duration) *TelnetChecker {
	return &TelnetChecker{
		timeout: timeout,
	}
}

// Check performs connectivity check using telnet for non-HTTP protocols
func (t *TelnetChecker) Check(ctx context.Context, target string) (int, error) {
	// Parse the target URL to extract host and port
	u, err := url.Parse(target)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}

	// Extract host and port
	host := u.Hostname()
	port := u.Port()
	
	// If no port is specified, use default ports based on scheme
	if port == "" {
		switch u.Scheme {
		case "ftp":
			port = "21"
		case "sftp", "ssh":
			port = "22"
		case "telnet":
			port = "23"
		case "smtp":
			port = "25"
		case "mysql":
			port = "3306"
		case "postgres", "postgresql":
			port = "5432"
		case "redis":
			port = "6379"
		case "mongodb":
			port = "27017"
		default:
			return 0, fmt.Errorf("no default port for scheme: %s", u.Scheme)
		}
	}

	// Create a dialer with timeout
	dialer := net.Dialer{
		Timeout: t.timeout,
	}

	// Use context for cancellation
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return 0, fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Connection successful
	return 200, nil // Return 200 to indicate success for non-HTTP protocols
}

// Protocol returns the protocol name
func (t *TelnetChecker) Protocol() string {
	return "telnet"
}

func New(cfg *config.Config) *Checker {
	restConfig := &rest.Config{
		RetryCount:    cfg.Retries,
		RetryWaitTime: time.Second,
		Timeout:       cfg.Timeout,
	}

	restClient := rest.NewClient(rest.WithRestConfig(*restConfig))

	// Initialize protocol checkers
	checkers := make(map[string]ProtocolChecker)
	checkers["http"] = NewHTTPChecker(restClient)
	checkers["https"] = NewHTTPChecker(restClient)
	checkers["ftp"] = NewTelnetChecker(cfg.Timeout)
	checkers["sftp"] = NewTelnetChecker(cfg.Timeout)
	checkers["ssh"] = NewTelnetChecker(cfg.Timeout)
	checkers["telnet"] = NewTelnetChecker(cfg.Timeout)
	checkers["smtp"] = NewTelnetChecker(cfg.Timeout)
	checkers["mysql"] = NewTelnetChecker(cfg.Timeout)
	checkers["postgres"] = NewTelnetChecker(cfg.Timeout)
	checkers["postgresql"] = NewTelnetChecker(cfg.Timeout)
	checkers["redis"] = NewTelnetChecker(cfg.Timeout)
	checkers["mongodb"] = NewTelnetChecker(cfg.Timeout)

	return &Checker{
		config:     cfg,
		restClient: restClient,
		results:    make(chan Result, len(cfg.Targets)*2),
		checkers:   checkers,
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
	// Parse URL to determine protocol
	u, err := url.Parse(targetURL)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}

	// Get the appropriate checker for the protocol
	checker, exists := c.checkers[u.Scheme]
	if !exists {
		return 0, fmt.Errorf("unsupported protocol: %s", u.Scheme)
	}

	// Perform the check using the appropriate protocol checker
	return checker.Check(ctx, targetURL)
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
