package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jasoet/url-exporter/internal/config"
	"github.com/stretchr/testify/assert"
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
	assert.Contains(t, err.Error(), "network error")
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