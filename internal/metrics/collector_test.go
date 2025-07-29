package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jasoet/url-exporter/internal/checker"
	"github.com/jasoet/url-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com", "https://test.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	assert.NotNil(t, collector)
	assert.Equal(t, cfg, collector.config)
	assert.Equal(t, chk, collector.checker)
	assert.NotNil(t, collector.lastResults)
	assert.Equal(t, 0, len(collector.lastResults))
	assert.NotNil(t, collector.counters)
	assert.Equal(t, 0, len(collector.counters))
	
	// Verify all metric descriptors are created
	assert.NotNil(t, collector.urlUp)
	assert.NotNil(t, collector.urlError)
	assert.NotNil(t, collector.urlResponseTime)
	assert.NotNil(t, collector.urlHTTPStatusCode)
	assert.NotNil(t, collector.urlCheckTotal)
	assert.NotNil(t, collector.urlStatusCodeTotal)
}

func TestNewCollector_MetricDescriptors(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Test urlUp descriptor
	assert.Contains(t, collector.urlUp.String(), "url_up")
	
	// Test urlError descriptor
	assert.Contains(t, collector.urlError.String(), "url_error")
	
	// Test urlResponseTime descriptor
	assert.Contains(t, collector.urlResponseTime.String(), "url_response_time_milliseconds")
	
	// Test urlHTTPStatusCode descriptor
	assert.Contains(t, collector.urlHTTPStatusCode.String(), "url_http_status_code")
	
	// Test urlCheckTotal descriptor
	assert.Contains(t, collector.urlCheckTotal.String(), "url_check_total")
	
	// Test urlStatusCodeTotal descriptor
	assert.Contains(t, collector.urlStatusCodeTotal.String(), "url_status_code_total")
}

func TestCollector_Describe(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)
	
	var descriptors []*prometheus.Desc
	for desc := range ch {
		descriptors = append(descriptors, desc)
	}
	
	assert.Equal(t, 6, len(descriptors))
	
	// Verify all expected descriptors are present
	expectedDescs := []*prometheus.Desc{
		collector.urlUp,
		collector.urlError,
		collector.urlResponseTime,
		collector.urlHTTPStatusCode,
		collector.urlCheckTotal,
		collector.urlStatusCodeTotal,
	}
	
	for _, expected := range expectedDescs {
		found := false
		for _, actual := range descriptors {
			if expected == actual {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected descriptor not found: %v", expected)
	}
}

func TestCollector_Collect_EmptyResults(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)
	
	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	
	assert.Equal(t, 0, len(metrics))
}

func TestCollector_Collect_SuccessfulResult(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Add a successful result
	result := &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com",
		Path:         "/",
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		Error:        nil,
		Timestamp:    time.Now(),
	}
	
	collector.mutex.Lock()
	collector.lastResults[result.URL] = result
	// Simulate counter tracking
	collector.counters[result.URL] = map[string]int{"200": 1}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 20)
	collector.Collect(ch)
	close(ch)
	
	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	
	// Should have 6 metrics: url_up, url_error, url_response_time, url_http_status_code, url_check_total, url_status_code_total
	assert.Equal(t, 6, len(metrics))
	
	// Verify metrics values
	for _, metric := range metrics {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		// Check labels - counters have 5 labels, gauges have 4
		labels := dto.GetLabel()
		
		var urlLabel, hostLabel, pathLabel, instanceLabel string
		for _, label := range labels {
			switch label.GetName() {
			case "url":
				urlLabel = label.GetValue()
			case "host":
				hostLabel = label.GetValue()
			case "path":
				pathLabel = label.GetValue()
			case "instance":
				instanceLabel = label.GetValue()
			}
		}
		
		assert.Equal(t, "https://example.com", urlLabel)
		assert.Equal(t, "https://example.com", hostLabel)
		assert.Equal(t, "/", pathLabel)
		assert.Equal(t, "test-instance", instanceLabel)
		
		// Check metric values based on type
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_up") {
			assert.Equal(t, float64(1), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_error") {
			assert.Equal(t, float64(0), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_response_time_milliseconds") {
			assert.Equal(t, float64(150), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_http_status_code") {
			assert.Equal(t, float64(200), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_check_total") {
			// Counter metric should have 6 labels including status_code and protocol
			assert.Equal(t, 6, len(labels))
			assert.Equal(t, float64(1), dto.GetCounter().GetValue())
		} else if strings.Contains(descStr, "url_status_code_total") {
			// Counter metric should have 6 labels including status_code and protocol
			assert.Equal(t, 6, len(labels))
			assert.Equal(t, float64(1), dto.GetCounter().GetValue())
		}
	}
}

func TestCollector_Collect_ErrorResult(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Add an error result
	result := &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com", 
		Path:         "/",
		StatusCode:   0,
		ResponseTime: 0,
		Error:        errors.New("connection refused"),
		Timestamp:    time.Now(),
	}
	
	collector.mutex.Lock()
	collector.lastResults[result.URL] = result
	// Simulate counter tracking for error
	collector.counters[result.URL] = map[string]int{"error": 1}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 20)
	collector.Collect(ch)
	close(ch)
	
	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	
	// Should have 4 metrics: url_up, url_error (gauges) + url_check_total, url_status_code_total (counters)
	assert.Equal(t, 4, len(metrics))
	
	// Verify metrics values
	for _, metric := range metrics {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_up") {
			assert.Equal(t, float64(0), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_error") {
			assert.Equal(t, float64(1), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_check_total") || strings.Contains(descStr, "url_status_code_total") {
			// Counter metrics should have "error" as status_code
			labels := dto.GetLabel()
			var statusCodeLabel string
			for _, label := range labels {
				if label.GetName() == "status_code" {
					statusCodeLabel = label.GetValue()
					break
				}
			}
			assert.Equal(t, "error", statusCodeLabel)
			assert.Equal(t, float64(1), dto.GetCounter().GetValue())
		}
	}
}

func TestCollector_Collect_HTTPErrorResult(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Add an HTTP error result (4xx/5xx status codes)
	result := &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com",
		Path:         "/notfound",
		StatusCode:   404,
		ResponseTime: 100 * time.Millisecond,
		Error:        nil,
		Timestamp:    time.Now(),
	}
	
	collector.mutex.Lock()
	collector.lastResults[result.URL] = result
	// Simulate counter tracking for 404
	collector.counters[result.URL] = map[string]int{"404": 1}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 20)
	collector.Collect(ch)
	close(ch)
	
	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	
	// Should have 6 metrics: url_up, url_error, url_response_time, url_http_status_code + counters
	assert.Equal(t, 6, len(metrics))
	
	// Verify metrics values
	for _, metric := range metrics {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_up") {
			// url_up should be 0 for non-2xx status codes
			assert.Equal(t, float64(0), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_error") {
			// url_error should be 0 for HTTP responses (no network error)
			assert.Equal(t, float64(0), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_response_time_milliseconds") {
			assert.Equal(t, float64(100), dto.GetGauge().GetValue())
		} else if strings.Contains(descStr, "url_http_status_code") {
			assert.Equal(t, float64(404), dto.GetGauge().GetValue())
		}
	}
}

func TestCollector_Collect_MultipleResults(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com", "https://test.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Add multiple results
	results := []*checker.Result{
		{
			URL:          "https://example.com",
			Host:         "https://example.com",
			Path:         "/",
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			Error:        nil,
			Timestamp:    time.Now(),
		},
		{
			URL:          "https://test.com",
			Host:         "https://test.com",
			Path:         "/api",
			StatusCode:   0,
			ResponseTime: 0,
			Error:        errors.New("timeout"),
			Timestamp:    time.Now(),
		},
	}
	
	collector.mutex.Lock()
	for _, result := range results {
		collector.lastResults[result.URL] = result
	}
	// Simulate counters
	collector.counters["https://example.com"] = map[string]int{"200": 1}
	collector.counters["https://test.com"] = map[string]int{"error": 1}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 30)
	collector.Collect(ch)
	close(ch)
	
	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	
	// Should have 10 metrics total: 
	// - example.com: 4 gauges + 2 counters = 6
	// - test.com: 2 gauges + 2 counters = 4
	assert.Equal(t, 10, len(metrics))
	
	// Count metrics by URL
	urlMetrics := make(map[string]int)
	for _, metric := range metrics {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		labels := dto.GetLabel()
		for _, label := range labels {
			if label.GetName() == "url" {
				urlMetrics[label.GetValue()]++
				break
			}
		}
	}
	
	assert.Equal(t, 6, urlMetrics["https://example.com"]) // Success: 4 gauges + 2 counters
	assert.Equal(t, 4, urlMetrics["https://test.com"])    // Error: 2 gauges + 2 counters
}

func TestCollector_Register_Success(t *testing.T) {
	// Create a new registry to avoid conflicts
	registry := prometheus.NewRegistry()
	
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Register with custom registry
	err := registry.Register(collector)
	assert.NoError(t, err)
	
	// Verify it's registered by checking if it can collect metrics
	gathered, err := registry.Gather()
	assert.NoError(t, err)
	assert.NotNil(t, gathered)
}

func TestCollector_Register_GlobalRegistry(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance-global",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// This will use the global registry
	err := collector.Register()
	if err != nil {
		// Registration might fail if collector is already registered in global registry
		// This is expected in test environments
		assert.Contains(t, err.Error(), "failed to register collector")
	}
}

func TestCollector_Start_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Start should return when context is cancelled
	start := time.Now()
	collector.Start(ctx)
	elapsed := time.Since(start)
	
	// Should return reasonably quickly after context timeout
	assert.Less(t, elapsed, 200*time.Millisecond)
}

func TestCollector_Start_ProcessResults(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	// Start collector in background
	go collector.Start(ctx)
	
	// Give some time for Start to initialize counters
	time.Sleep(50 * time.Millisecond)
	
	// Verify counters are initialized for configured targets
	collector.mutex.RLock()
	_, exists := collector.counters["https://example.com"]
	collector.mutex.RUnlock()
	assert.True(t, exists, "Counters should be initialized for configured targets")
	
	// Wait for context to timeout
	<-ctx.Done()
	
	// Verify the lastResults map is still accessible (no race conditions)
	collector.mutex.RLock()
	resultsCount := len(collector.lastResults)
	countersCount := len(collector.counters)
	collector.mutex.RUnlock()
	
	// Should be 0 results since we couldn't inject results, but counters should be initialized
	assert.Equal(t, 0, resultsCount)
	assert.Equal(t, 1, countersCount)
}

func TestCollector_ThreadSafety(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com", "https://test.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Simulate concurrent access to lastResults
	done := make(chan bool, 2)
	
	// Goroutine 1: Write results
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			result := &checker.Result{
				URL:          "https://example.com",
				Host:         "https://example.com",
				Path:         "/",
				StatusCode:   200 + i%100,
				ResponseTime: time.Duration(i) * time.Millisecond,
				Error:        nil,
				Timestamp:    time.Now(),
			}
			
			collector.mutex.Lock()
			collector.lastResults[result.URL] = result
			collector.mutex.Unlock()
			
			time.Sleep(time.Microsecond)
		}
	}()
	
	// Goroutine 2: Read results (Collect)
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			ch := make(chan prometheus.Metric, 10)
			collector.Collect(ch)
			close(ch)
			
			// Drain channel
			for range ch {
			}
			
			time.Sleep(time.Microsecond)
		}
	}()
	
	// Wait for both goroutines to complete
	<-done
	<-done
	
	// Should complete without race conditions or panics
	assert.True(t, true) // Test passes if we reach here without panicking
}

func TestCollector_URLErrorMetric_NetworkError(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",  
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Test network error case
	result := &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com",
		Path:         "/",
		StatusCode:   0,
		ResponseTime: 0,
		Error:        errors.New("dial tcp: connection refused"),
		Timestamp:    time.Now(),
	}
	
	collector.mutex.Lock()
	collector.lastResults[result.URL] = result
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)
	
	var urlUpValue, urlErrorValue float64
	var foundUrlUp, foundUrlError bool
	
	for metric := range ch {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_up") {
			urlUpValue = dto.GetGauge().GetValue()
			foundUrlUp = true
		} else if strings.Contains(descStr, "url_error") {
			urlErrorValue = dto.GetGauge().GetValue()
			foundUrlError = true
		}
	}
	
	assert.True(t, foundUrlUp, "url_up metric should be present")
	assert.True(t, foundUrlError, "url_error metric should be present")
	assert.Equal(t, float64(0), urlUpValue, "url_up should be 0 for network errors")
	assert.Equal(t, float64(1), urlErrorValue, "url_error should be 1 for network errors")
}

func TestCollector_URLErrorMetric_HTTPError(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Test HTTP error case (server responds but with error status)
	result := &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com",
		Path:         "/notfound",
		StatusCode:   500,
		ResponseTime: 200 * time.Millisecond,
		Error:        nil, // No network error, server responded
		Timestamp:    time.Now(),
	}
	
	collector.mutex.Lock()
	collector.lastResults[result.URL] = result
	collector.counters[result.URL] = map[string]int{"500": 1}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 20)
	collector.Collect(ch)
	close(ch)
	
	var urlUpValue, urlErrorValue float64
	var foundUrlUp, foundUrlError bool
	
	for metric := range ch {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_up") {
			urlUpValue = dto.GetGauge().GetValue()
			foundUrlUp = true
		} else if strings.Contains(descStr, "url_error") {
			urlErrorValue = dto.GetGauge().GetValue()
			foundUrlError = true
		}
	}
	
	assert.True(t, foundUrlUp, "url_up metric should be present")
	assert.True(t, foundUrlError, "url_error metric should be present")
	assert.Equal(t, float64(0), urlUpValue, "url_up should be 0 for non-2xx status codes")
	assert.Equal(t, float64(0), urlErrorValue, "url_error should be 0 for HTTP responses (no network error)")
}

func TestCollector_CounterPersistence(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Add a successful result
	result := &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com",
		Path:         "/",
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		Error:        nil,
		Timestamp:    time.Now(),
	}
	
	// Simulate multiple checks
	collector.mutex.Lock()
	collector.lastResults[result.URL] = result
	collector.counters[result.URL] = map[string]int{
		"200": 10,  // 10 successful checks
		"404": 3,   // 3 not found
		"500": 2,   // 2 server errors
		"error": 1, // 1 network error
	}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 30)
	collector.Collect(ch)
	close(ch)
	
	counterMetrics := make(map[string]float64)
	
	for metric := range ch {
		dto := &dto.Metric{}
		err := metric.Write(dto)
		require.NoError(t, err)
		
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_check_total") || strings.Contains(descStr, "url_status_code_total") {
			labels := dto.GetLabel()
			var statusCode string
			for _, label := range labels {
				if label.GetName() == "status_code" {
					statusCode = label.GetValue()
					break
				}
			}
			
			if strings.Contains(descStr, "url_check_total") {
				counterMetrics["check_"+statusCode] = dto.GetCounter().GetValue()
			} else {
				counterMetrics["status_"+statusCode] = dto.GetCounter().GetValue()
			}
		}
	}
	
	// Verify all counters are exposed correctly
	assert.Equal(t, float64(10), counterMetrics["check_200"])
	assert.Equal(t, float64(10), counterMetrics["status_200"])
	assert.Equal(t, float64(3), counterMetrics["check_404"])
	assert.Equal(t, float64(3), counterMetrics["status_404"])
	assert.Equal(t, float64(2), counterMetrics["check_500"])
	assert.Equal(t, float64(2), counterMetrics["status_500"])
	assert.Equal(t, float64(1), counterMetrics["check_error"])
	assert.Equal(t, float64(1), counterMetrics["status_error"])
}

func TestCollector_MultipleURLsWithCounters(t *testing.T) {
	cfg := &config.Config{
		Targets:    []string{"https://example.com", "https://test.com", "https://api.com"},
		InstanceID: "test-instance",
	}
	
	chk := checker.New(cfg)
	collector := NewCollector(cfg, chk)
	
	// Add results for multiple URLs
	collector.mutex.Lock()
	collector.lastResults["https://example.com"] = &checker.Result{
		URL:          "https://example.com",
		Host:         "https://example.com",
		Path:         "/",
		StatusCode:   200,
		ResponseTime: 100 * time.Millisecond,
		Error:        nil,
		Timestamp:    time.Now(),
	}
	collector.counters["https://example.com"] = map[string]int{"200": 5, "500": 1}
	
	collector.lastResults["https://test.com"] = &checker.Result{
		URL:          "https://test.com",
		Host:         "https://test.com",
		Path:         "/api",
		StatusCode:   404,
		ResponseTime: 200 * time.Millisecond,
		Error:        nil,
		Timestamp:    time.Now(),
	}
	collector.counters["https://test.com"] = map[string]int{"404": 3, "200": 7}
	
	collector.lastResults["https://api.com"] = &checker.Result{
		URL:          "https://api.com",
		Host:         "https://api.com",
		Path:         "/v1",
		StatusCode:   0,
		ResponseTime: 0,
		Error:        errors.New("timeout"),
		Timestamp:    time.Now(),
	}
	collector.counters["https://api.com"] = map[string]int{"error": 2, "200": 8}
	collector.mutex.Unlock()
	
	ch := make(chan prometheus.Metric, 50)
	collector.Collect(ch)
	close(ch)
	
	// Count metrics by type
	metricCounts := make(map[string]int)
	for metric := range ch {
		descStr := metric.Desc().String()
		if strings.Contains(descStr, "url_up") {
			metricCounts["url_up"]++
		} else if strings.Contains(descStr, "url_error") {
			metricCounts["url_error"]++
		} else if strings.Contains(descStr, "url_response_time") {
			metricCounts["url_response_time"]++
		} else if strings.Contains(descStr, "url_http_status_code") {
			metricCounts["url_http_status_code"]++
		} else if strings.Contains(descStr, "url_check_total") {
			metricCounts["url_check_total"]++
		} else if strings.Contains(descStr, "url_status_code_total") {
			metricCounts["url_status_code_total"]++
		}
	}
	
	// Should have 3 of each gauge metric (one per URL)
	assert.Equal(t, 3, metricCounts["url_up"])
	assert.Equal(t, 3, metricCounts["url_error"])
	assert.Equal(t, 2, metricCounts["url_response_time"]) // api.com has error, no response time
	assert.Equal(t, 2, metricCounts["url_http_status_code"]) // api.com has error, no status code
	
	// Counter metrics: example.com has 2 statuses, test.com has 2, api.com has 2
	assert.Equal(t, 6, metricCounts["url_check_total"])
	assert.Equal(t, 6, metricCounts["url_status_code_total"])
}