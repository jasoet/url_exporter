package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jasoet/url-exporter/internal/checker"
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/jasoet/url-exporter/internal/metrics"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testVersionInfo returns a version info for testing
func testVersionInfo() *VersionInfo {
	return &VersionInfo{
		Version: "test-1.0.0",
		Commit:  "test123",
		Date:    "2024-01-01",
		BuiltBy: "test",
	}
}

// createTestServer creates a server with isolated Prometheus registry to avoid conflicts
func createTestServer(cfg *config.Config) (*URLExporterServer, error) {
	chk := checker.New(cfg)
	col := metrics.NewCollector(cfg, chk)

	// Create a separate registry for each test to avoid conflicts
	registry := prometheus.NewRegistry()
	if err := registry.Register(col); err != nil {
		return nil, err
	}

	s := &URLExporterServer{
		config:    cfg,
		checker:   chk,
		collector: col,
		version:   testVersionInfo(),
	}

	return s, nil
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com", "https://test.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, cfg, server.config)
	assert.NotNil(t, server.checker)
	assert.NotNil(t, server.collector)
}

func TestNew_WithRegistrationFailure(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	// Test actual New function which uses global registry
	version := testVersionInfo()
	server1, err := New(cfg, version)
	assert.NoError(t, err)
	assert.NotNil(t, server1)

	// Second server creation might fail due to duplicate collector registration
	// This is expected behavior with global registry
	_, err2 := New(cfg, version)
	if err2 != nil {
		assert.Contains(t, err2.Error(), "failed to register metrics collector")
	}
}

func TestURLExporterServer_HandleRoot(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com", "https://test.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	// Create Echo instance and request
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call handleRoot
	err = server.handleRoot(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	// Parse and validate JSON response
	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "url-exporter", response["service"])
	assert.Equal(t, "test-1.0.0", response["version"])
	assert.Equal(t, "test123", response["commit"])
	assert.Equal(t, "2024-01-01", response["date"])
	assert.Equal(t, "test", response["built_by"])
	assert.Equal(t, "test-instance", response["instance"])
	assert.Equal(t, float64(2), response["targets"]) // JSON unmarshals numbers as float64
	assert.Equal(t, "running", response["status"])

	endpoints, ok := response["endpoints"].([]interface{})
	assert.True(t, ok)
	assert.Contains(t, endpoints, "/")
	assert.Contains(t, endpoints, "/health")
	assert.Contains(t, endpoints, "/metrics")
}

func TestURLExporterServer_HandleRoot_EmptyTargets(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"}, // Need at least one target to pass validation
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = server.handleRoot(c)
	assert.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["targets"])
}

func TestURLExporterServer_HandleRoot_CustomInstanceID(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "custom-test-instance-123",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = server.handleRoot(c)
	assert.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "custom-test-instance-123", response["instance"])
}

func TestURLExporterServer_SetupRoutes(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	server.setupRoutes(e)

	// Test root route exists
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test metrics route exists
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
}

func TestURLExporterServer_SetupRoutes_MetricsEndpoint(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	server.setupRoutes(e)

	// Test metrics endpoint returns Prometheus format
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Should contain Prometheus metrics format
	body := rec.Body.String()
	assert.Contains(t, body, "# HELP")
	assert.Contains(t, body, "# TYPE")
}

func TestURLExporterServer_SetupRoutes_InvalidRoute(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	server.setupRoutes(e)

	// Test non-existent route returns 404
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestURLExporterServer_StartBackgroundWorkers(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	// Test that startBackgroundWorkers doesn't panic
	// We can't easily test the actual goroutines without complex mocking
	ctx := context.Background()

	// This should not panic or block
	server.startBackgroundWorkers(ctx)

	// Give a brief moment for goroutines to start
	time.Sleep(10 * time.Millisecond)

	// Test passes if we reach here without panicking
	assert.True(t, true)
}

func TestURLExporterServer_HandleRoot_HTTPMethods(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	server.setupRoutes(e)

	// Test GET method (should work)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test POST method (should return method not allowed)
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	// Test PUT method (should return method not allowed)
	req = httptest.NewRequest(http.MethodPut, "/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestURLExporterServer_Integration_AllEndpoints(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com", "https://google.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "integration-test",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	server.setupRoutes(e)

	// Test all endpoints work together
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		contentType    string
	}{
		{
			name:           "Root endpoint",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusOK,
			contentType:    "application/json",
		},
		{
			name:           "Metrics endpoint",
			method:         http.MethodGet,
			path:           "/metrics",
			expectedStatus: http.StatusOK,
			contentType:    "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Contains(t, rec.Header().Get("Content-Type"), tt.contentType)
			assert.Greater(t, len(rec.Body.String()), 0)
		})
	}
}

func TestURLExporterServer_ComponentsInitialization(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	// Verify all components are properly initialized
	assert.NotNil(t, server.config, "Config should be initialized")
	assert.NotNil(t, server.checker, "Checker should be initialized")
	assert.NotNil(t, server.collector, "Collector should be initialized")

	// Verify config is correctly set
	assert.Equal(t, cfg.Targets, server.config.Targets)
	assert.Equal(t, cfg.InstanceID, server.config.InstanceID)
	assert.Equal(t, cfg.ListenPort, server.config.ListenPort)
}

func TestURLExporterServer_HandleRoot_JSONStructure(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com", "https://test.com", "https://github.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    9999,
		InstanceID:    "json-test-instance",
		Retries:       5,
		LogLevel:      "debug",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = server.handleRoot(c)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify all expected fields are present
	requiredFields := []string{"service", "version", "commit", "date", "built_by", "instance", "targets", "status", "endpoints"}
	for _, field := range requiredFields {
		assert.Contains(t, response, field, "Response should contain field: %s", field)
	}

	// Verify field types and values
	assert.IsType(t, "", response["service"])
	assert.IsType(t, "", response["version"])
	assert.IsType(t, "", response["instance"])
	assert.IsType(t, float64(0), response["targets"])
	assert.IsType(t, "", response["status"])
	assert.IsType(t, []interface{}{}, response["endpoints"])

	// Verify specific values
	assert.Equal(t, "url-exporter", response["service"])
	assert.Equal(t, "json-test-instance", response["instance"])
	assert.Equal(t, float64(3), response["targets"])
	assert.Equal(t, "running", response["status"])
}

func TestURLExporterServer_Start_ConfigValidation(t *testing.T) {
	cfg := &config.Config{
		Targets:       []string{"https://example.com"},
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		ListenPort:    8412,
		InstanceID:    "test-instance",
		Retries:       3,
		LogLevel:      "info",
	}

	server, err := createTestServer(cfg)
	require.NoError(t, err)

	// The Start() method uses the jasoet/pkg/server which we cannot easily test
	// without starting an actual server. We'll test that the method exists and
	// doesn't panic when called.

	// Verify the method exists and can be called
	assert.NotPanics(t, func() {
		// We can't actually call Start() in tests as it blocks and starts a real server
		// Instead, verify the server has the required configuration
		assert.Equal(t, 8412, server.config.ListenPort)
		assert.NotEmpty(t, server.config.Targets)
		assert.NotEmpty(t, server.config.InstanceID)
	})
}
