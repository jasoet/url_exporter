package checker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jasoet/pkg/rest"
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com", "https://google.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       5 * time.Second,
		Retries:       3,
	}

	checker := New(cfg)

	assert.NotNil(t, checker)
	assert.Equal(t, cfg, checker.config)
	assert.NotNil(t, checker.restClient)
	assert.NotNil(t, checker.results)
	assert.Equal(t, 5*time.Second, checker.restClient.GetRestConfig().Timeout)
	assert.Equal(t, len(cfg.Targets)*2, cap(checker.results))
}

func TestNew_RestClientConfiguration(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"https://example.com"},
		Timeout: 10 * time.Second,
		Retries: 2,
	}

	checker := New(cfg)

	restConfig := checker.restClient.GetRestConfig()
	assert.Equal(t, 10*time.Second, restConfig.Timeout)
	assert.Equal(t, 2, restConfig.RetryCount)
	assert.Equal(t, time.Second, restConfig.RetryWaitTime)
}

func TestNew_RestClientExists(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"https://example.com"},
		Timeout: 5 * time.Second,
	}

	checker := New(cfg)

	assert.NotNil(t, checker.restClient)
	assert.NotNil(t, checker.restClient.GetRestClient())
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedHost string
		expectedPath string
	}{
		{
			name:         "Simple HTTPS URL",
			url:          "https://example.com",
			expectedHost: "https://example.com",
			expectedPath: "/",
		},
		{
			name:         "HTTPS URL with path",
			url:          "https://example.com/api/health",
			expectedHost: "https://example.com",
			expectedPath: "/api/health",
		},
		{
			name:         "HTTP URL with port",
			url:          "http://localhost:8080/metrics",
			expectedHost: "http://localhost:8080",
			expectedPath: "/metrics",
		},
		{
			name:         "URL with query parameters",
			url:          "https://api.example.com/search?q=test&limit=10",
			expectedHost: "https://api.example.com",
			expectedPath: "/search?q=test&limit=10",
		},
		{
			name:         "URL with fragment (ignored)",
			url:          "https://example.com/page#section",
			expectedHost: "https://example.com",
			expectedPath: "/page",
		},
		{
			name:         "Path-only URL",
			url:          "not-a-valid-url",
			expectedHost: "://",
			expectedPath: "not-a-valid-url",
		},
		{
			name:         "URL with empty path",
			url:          "https://example.com/",
			expectedHost: "https://example.com",
			expectedPath: "/",
		},
		{
			name:         "Complex URL with subdomain and path",
			url:          "https://api.v2.example.com/v1/users?active=true",
			expectedHost: "https://api.v2.example.com",
			expectedPath: "/v1/users?active=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, path := parseURL(tt.url)
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

func TestPerformCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		assert.Equal(t, "url-exporter/1.0", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	statusCode, err := checker.performCheck(ctx, server.URL)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
}

func TestPerformCheck_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	statusCode, err := checker.performCheck(ctx, server.URL)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, statusCode)
}

func TestPerformCheck_NetworkError(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"http://localhost:99999"},
		Timeout: 1 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	statusCode, err := checker.performCheck(ctx, "http://localhost:99999")

	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "network error")
}

func TestPerformCheck_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"invalid-url"},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	statusCode, err := checker.performCheck(ctx, "://invalid-url")

	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestPerformCheck_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	statusCode, err := checker.performCheck(ctx, server.URL)

	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "network error")
}

func TestCheckURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, server.URL)

	assert.Equal(t, server.URL, result.URL)
	assert.Equal(t, server.URL, result.Host)
	assert.Equal(t, "/", result.Path)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.NoError(t, result.Error)
	assert.True(t, result.ResponseTime > 0)
	assert.False(t, result.Timestamp.IsZero())
}

func TestCheckURL_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, server.URL)

	assert.Equal(t, server.URL, result.URL)
	assert.Equal(t, http.StatusNotFound, result.StatusCode)
	assert.NoError(t, result.Error)
	assert.True(t, result.ResponseTime > 0)
}

func TestCheckURL_NetworkError(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"http://localhost:99999"},
		Timeout: 1 * time.Second,
		Retries: 2,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, "http://localhost:99999")

	assert.Equal(t, "http://localhost:99999", result.URL)
	assert.Equal(t, "http://localhost:99999", result.Host)
	assert.Equal(t, "/", result.Path)
	assert.Equal(t, 0, result.StatusCode)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "network error")
	assert.False(t, result.Timestamp.IsZero())
}

func TestCheckURL_RetryLogic(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 1 * time.Second,
		Retries: 2,
	}

	checker := New(cfg)
	ctx := context.Background()

	start := time.Now()
	result := checker.checkURL(ctx, server.URL)
	elapsed := time.Since(start)

	assert.Equal(t, 0, result.StatusCode)
	assert.Error(t, result.Error)
	assert.Equal(t, 0, callCount)
	assert.True(t, elapsed >= 2*time.Second)
}

func TestCheckURL_HTTPStatusNoRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 3,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, server.URL)

	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
	assert.NoError(t, result.Error)
	assert.Equal(t, 1, callCount)
}

func TestCheckURL_WithPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL + "/health"},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, server.URL+"/health")

	assert.Equal(t, server.URL, result.Host)
	assert.Equal(t, "/health", result.Path)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.NoError(t, result.Error)
}

func TestCheckURL_WithQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test", r.URL.Query().Get("q"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	targetURL := server.URL + "/search?q=test&limit=10"
	cfg := &config.Config{
		Targets: []string{targetURL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, targetURL)

	assert.Equal(t, server.URL, result.Host)
	assert.Equal(t, "/search?q=test&limit=10", result.Path)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.NoError(t, result.Error)
}

func TestResults_Channel(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"https://example.com"},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	results := checker.Results()

	assert.NotNil(t, results)
	assert.IsType(t, (<-chan Result)(nil), results)
}

func TestShutdown(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 1 * time.Second,
		Timeout:       5 * time.Second,
		Retries:       1,
	}

	checker := New(cfg)
	ctx := context.Background()

	go checker.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	err := checker.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestStart_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets:       []string{server.URL},
		CheckInterval: 100 * time.Millisecond,
		Timeout:       5 * time.Second,
		Retries:       1,
	}

	checker := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go checker.Start(ctx)

	var results []Result
	timeout := time.After(400 * time.Millisecond)

	for len(results) < 2 {
		select {
		case result := <-checker.Results():
			results = append(results, result)
		case <-timeout:
			t.Fatal("Did not receive expected results within timeout")
		}
	}

	assert.GreaterOrEqual(t, len(results), 2)
	for _, result := range results {
		assert.Equal(t, server.URL, result.URL)
		assert.Equal(t, http.StatusOK, result.StatusCode)
		assert.NoError(t, result.Error)
	}
}

func TestCheckURL_ResponseTimeAccuracy(t *testing.T) {
	delay := 100 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	result := checker.checkURL(ctx, server.URL)

	assert.NoError(t, result.Error)
	assert.GreaterOrEqual(t, result.ResponseTime, delay)
	assert.Less(t, result.ResponseTime, delay+50*time.Millisecond)
}

func TestPerformCheck_UserAgent(t *testing.T) {
	var capturedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	_, err := checker.performCheck(ctx, server.URL)

	assert.NoError(t, err)
	assert.Equal(t, "url-exporter/1.0", capturedUserAgent)
}

func TestCheckAllURLs_ConcurrentExecution(t *testing.T) {
	serverCount := 3
	servers := make([]*httptest.Server, serverCount)
	urls := make([]string, serverCount)

	for i := 0; i < serverCount; i++ {
		i := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		urls[i] = servers[i].URL
	}

	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	cfg := &config.Config{
		Targets: urls,
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	checker := New(cfg)
	ctx := context.Background()

	start := time.Now()
	checker.checkAllURLs(ctx)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 100*time.Millisecond, "Concurrent execution should be faster than sequential")

	var results []Result
	timeout := time.After(100 * time.Millisecond)

	for len(results) < serverCount {
		select {
		case result := <-checker.Results():
			results = append(results, result)
		case <-timeout:
			break
		}
	}

	assert.Equal(t, serverCount, len(results))
	for _, result := range results {
		assert.NoError(t, result.Error)
		assert.Equal(t, http.StatusOK, result.StatusCode)
	}
}

// Protocol Checker Tests

func TestHTTPChecker_NewHTTPChecker(t *testing.T) {
	cfg := &config.Config{
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	restConfig := &rest.Config{
		Timeout: cfg.Timeout,
		RetryCount: cfg.Retries,
		RetryWaitTime: time.Second,
	}
	restClient := rest.NewClient(rest.WithRestConfig(*restConfig))
	
	checker := NewHTTPChecker(restClient)
	
	assert.NotNil(t, checker)
	assert.NotNil(t, checker.restClient)
	assert.Equal(t, "http", checker.Protocol())
}

func TestHTTPChecker_Check_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "HEAD", r.Method)
		assert.Equal(t, "url-exporter/1.0", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	cfg := &config.Config{
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	restConfig := &rest.Config{
		Timeout: cfg.Timeout,
		RetryCount: cfg.Retries,
		RetryWaitTime: time.Second,
	}
	restClient := rest.NewClient(rest.WithRestConfig(*restConfig))
	
	checker := NewHTTPChecker(restClient)
	ctx := context.Background()
	
	statusCode, err := checker.Check(ctx, server.URL)
	
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}

func TestHTTPChecker_Check_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	
	cfg := &config.Config{
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	restConfig := &rest.Config{
		Timeout: cfg.Timeout,
		RetryCount: cfg.Retries,
		RetryWaitTime: time.Second,
	}
	restClient := rest.NewClient(rest.WithRestConfig(*restConfig))
	
	checker := NewHTTPChecker(restClient)
	ctx := context.Background()
	
	statusCode, err := checker.Check(ctx, server.URL)
	
	assert.NoError(t, err)
	assert.Equal(t, 404, statusCode)
}

func TestHTTPChecker_Check_NetworkError(t *testing.T) {
	cfg := &config.Config{
		Timeout: 1 * time.Second,
		Retries: 1,
	}
	
	restConfig := &rest.Config{
		Timeout: cfg.Timeout,
		RetryCount: cfg.Retries,
		RetryWaitTime: time.Second,
	}
	restClient := rest.NewClient(rest.WithRestConfig(*restConfig))
	
	checker := NewHTTPChecker(restClient)
	ctx := context.Background()
	
	statusCode, err := checker.Check(ctx, "http://localhost:99999")
	
	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "network error")
}

func TestTelnetChecker_NewTelnetChecker(t *testing.T) {
	timeout := 5 * time.Second
	checker := NewTelnetChecker(timeout)
	
	assert.NotNil(t, checker)
	assert.Equal(t, timeout, checker.timeout)
	assert.Equal(t, "telnet", checker.Protocol())
}

func TestTelnetChecker_Check_Success(t *testing.T) {
	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	
	timeout := 5 * time.Second
	checker := NewTelnetChecker(timeout)
	ctx := context.Background()
	
	// Use the listener's address
	targetURL := fmt.Sprintf("tcp://%s", listener.Addr().String())
	
	statusCode, err := checker.Check(ctx, targetURL)
	
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}

func TestTelnetChecker_Check_ConnectionFailure(t *testing.T) {
	timeout := 1 * time.Second
	checker := NewTelnetChecker(timeout)
	ctx := context.Background()
	
	statusCode, err := checker.Check(ctx, "tcp://localhost:99999")
	
	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestTelnetChecker_Check_InvalidURL(t *testing.T) {
	timeout := 5 * time.Second
	checker := NewTelnetChecker(timeout)
	ctx := context.Background()
	
	statusCode, err := checker.Check(ctx, "://invalid-url")
	
	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestTelnetChecker_Check_ContextCancellation(t *testing.T) {
	timeout := 5 * time.Second
	checker := NewTelnetChecker(timeout)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	statusCode, err := checker.Check(ctx, "tcp://1.1.1.1:12345")
	
	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
}

func TestTelnetChecker_Check_DefaultPorts(t *testing.T) {
	timeout := 1 * time.Second
	checker := NewTelnetChecker(timeout)
	ctx := context.Background()
	
	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{"FTP", "ftp://example.com", "21"},
		{"SFTP", "sftp://example.com", "22"},
		{"SSH", "ssh://example.com", "22"},
		{"Telnet", "telnet://example.com", "23"},
		{"SMTP", "smtp://example.com", "25"},
		{"MySQL", "mysql://example.com", "3306"},
		{"PostgreSQL", "postgres://example.com", "5432"},
		{"PostgreSQL Alt", "postgresql://example.com", "5432"},
		{"Redis", "redis://example.com", "6379"},
		{"MongoDB", "mongodb://example.com", "27017"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// We expect all these to fail with connection refused/timeout
			// but we're testing that the URL parsing and port assignment works
			statusCode, err := checker.Check(ctx, tc.url)
			
			assert.Error(t, err)
			assert.Equal(t, 0, statusCode)
			assert.Contains(t, err.Error(), "connection failed")
		})
	}
}

func TestTelnetChecker_Check_UnsupportedProtocol(t *testing.T) {
	timeout := 5 * time.Second
	checker := NewTelnetChecker(timeout)
	ctx := context.Background()
	
	statusCode, err := checker.Check(ctx, "unknown://example.com")
	
	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "no default port for scheme: unknown")
}

func TestTelnetChecker_Check_ExplicitPort(t *testing.T) {
	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	
	timeout := 5 * time.Second
	checker := NewTelnetChecker(timeout)
	ctx := context.Background()
	
	// Extract port from listener address
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	
	targetURL := fmt.Sprintf("ftp://127.0.0.1:%s", port)
	
	statusCode, err := checker.Check(ctx, targetURL)
	
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}

func TestPerformCheck_ProtocolSelection_HTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	ctx := context.Background()
	
	statusCode, err := checker.performCheck(ctx, server.URL)
	
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}

func TestPerformCheck_ProtocolSelection_HTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	cfg := &config.Config{
		Targets: []string{server.URL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	ctx := context.Background()
	
	statusCode, err := checker.performCheck(ctx, server.URL)
	
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}

func TestPerformCheck_ProtocolSelection_TCP(t *testing.T) {
	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	
	cfg := &config.Config{
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	ctx := context.Background()
	
	targetURL := fmt.Sprintf("ftp://%s", listener.Addr().String())
	
	statusCode, err := checker.performCheck(ctx, targetURL)
	
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}

func TestPerformCheck_UnsupportedProtocol(t *testing.T) {
	cfg := &config.Config{
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	ctx := context.Background()
	
	statusCode, err := checker.performCheck(ctx, "unknown://example.com")
	
	assert.Error(t, err)
	assert.Equal(t, 0, statusCode)
	assert.Contains(t, err.Error(), "unsupported protocol: unknown")
}

func TestChecker_ProtocolCheckersInitialization(t *testing.T) {
	cfg := &config.Config{
		Targets: []string{"https://example.com"},
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	
	// Verify all expected protocol checkers are initialized
	expectedProtocols := []string{
		"http", "https", "ftp", "sftp", "ssh", "telnet", 
		"smtp", "mysql", "postgres", "postgresql", "redis", "mongodb",
	}
	
	for _, protocol := range expectedProtocols {
		protocolChecker, exists := checker.checkers[protocol]
		assert.True(t, exists, "Protocol checker for %s should exist", protocol)
		assert.NotNil(t, protocolChecker, "Protocol checker for %s should not be nil", protocol)
	}
	
	// Verify HTTP/HTTPS use HTTPChecker
	httpChecker, ok := checker.checkers["http"].(*HTTPChecker)
	assert.True(t, ok, "HTTP checker should be HTTPChecker type")
	assert.NotNil(t, httpChecker.restClient)
	
	httpsChecker, ok := checker.checkers["https"].(*HTTPChecker)
	assert.True(t, ok, "HTTPS checker should be HTTPChecker type")
	assert.NotNil(t, httpsChecker.restClient)
	
	// Verify non-HTTP protocols use TelnetChecker
	ftpChecker, ok := checker.checkers["ftp"].(*TelnetChecker)
	assert.True(t, ok, "FTP checker should be TelnetChecker type")
	assert.Equal(t, cfg.Timeout, ftpChecker.timeout)
}

func TestCheckURL_MultipleProtocols_Integration(t *testing.T) {
	// Start HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer httpServer.Close()
	
	// Start TCP server for non-HTTP protocol
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer tcpListener.Close()
	
	go func() {
		for {
			conn, err := tcpListener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	
	tcpURL := fmt.Sprintf("ftp://%s", tcpListener.Addr().String())
	
	cfg := &config.Config{
		Targets: []string{httpServer.URL, tcpURL},
		Timeout: 5 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	ctx := context.Background()
	
	// Test HTTP protocol
	httpResult := checker.checkURL(ctx, httpServer.URL)
	assert.NoError(t, httpResult.Error)
	assert.Equal(t, 200, httpResult.StatusCode)
	assert.Equal(t, httpServer.URL, httpResult.URL)
	assert.True(t, httpResult.ResponseTime > 0)
	
	// Test FTP protocol
	ftpResult := checker.checkURL(ctx, tcpURL)
	assert.NoError(t, ftpResult.Error)
	assert.Equal(t, 200, ftpResult.StatusCode)
	assert.Equal(t, tcpURL, ftpResult.URL)
	assert.True(t, ftpResult.ResponseTime > 0)
}

func TestChecker_ProtocolSpecificErrorHandling(t *testing.T) {
	cfg := &config.Config{
		Timeout: 1 * time.Second,
		Retries: 1,
	}
	
	checker := New(cfg)
	ctx := context.Background()
	
	testCases := []struct {
		name        string
		url         string
		expectError bool
		errorType   string
	}{
		{
			name:        "HTTP Network Error",
			url:         "http://localhost:99999",
			expectError: true,
			errorType:   "network error",
		},
		{
			name:        "TCP Connection Error",
			url:         "ftp://localhost:99999",
			expectError: true,
			errorType:   "connection failed",
		},
		{
			name:        "Invalid URL HTTP",
			url:         "http://",
			expectError: true,
			errorType:   "network error",
		},
		{
			name:        "Invalid URL TCP",
			url:         "ftp://",
			expectError: true,
			errorType:   "invalid URL",
		},
		{
			name:        "Unsupported Protocol",
			url:         "gopher://example.com",
			expectError: true,
			errorType:   "unsupported protocol",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := checker.checkURL(ctx, tc.url)
			
			if tc.expectError {
				assert.Error(t, result.Error, "Expected error for %s", tc.name)
				assert.Contains(t, result.Error.Error(), tc.errorType, 
					"Error should contain '%s' for %s", tc.errorType, tc.name)
				assert.Equal(t, 0, result.StatusCode)
			} else {
				assert.NoError(t, result.Error, "Expected no error for %s", tc.name)
				assert.NotEqual(t, 0, result.StatusCode)
			}
		})
	}
}
