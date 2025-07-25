package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_WithDefaultConfig(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(cfg.Targets) != 2 {
		t.Errorf("Expected 2 default targets, got %d", len(cfg.Targets))
	}

	expectedTargets := []string{"https://google.com", "https://github.com"}
	for i, target := range expectedTargets {
		if i < len(cfg.Targets) && cfg.Targets[i] != target {
			t.Errorf("Target %d: expected %q, got %q", i, target, cfg.Targets[i])
		}
	}

	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("CheckInterval: expected %v, got %v", 30*time.Second, cfg.CheckInterval)
	}

	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout: expected %v, got %v", 10*time.Second, cfg.Timeout)
	}

	if cfg.ListenPort != 8412 {
		t.Errorf("ListenPort: expected %d, got %d", 8412, cfg.ListenPort)
	}

	if cfg.Retries != 3 {
		t.Errorf("Retries: expected %d, got %d", 3, cfg.Retries)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: expected %q, got %q", "info", cfg.LogLevel)
	}

	if cfg.InstanceID == "" {
		t.Error("InstanceID should be set to hostname when empty")
	}
}

func TestLoad_WithTargetsEnvironmentOverride(t *testing.T) {
	clearEnv(t)

	t.Setenv("URL_TARGETS", "https://example.com,https://test.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expectedTargets := []string{"https://example.com", "https://test.com"}
	if len(cfg.Targets) != len(expectedTargets) {
		t.Fatalf("Expected %d targets, got %d", len(expectedTargets), len(cfg.Targets))
	}

	for i, target := range cfg.Targets {
		if target != expectedTargets[i] {
			t.Errorf("Target %d: expected %q, got %q", i, expectedTargets[i], target)
		}
	}

	// Other values should remain defaults when not overridden
	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("CheckInterval should use default: expected %v, got %v", 30*time.Second, cfg.CheckInterval)
	}
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	clearEnv(t)

	// Use correct environment variable names (no underscores in field names)
	envVars := map[string]string{
		"URL_TARGETS":        "https://example.com,https://test.com",
		"URL_CHECKINTERVAL":  "60s",
		"URL_TIMEOUT":        "15s",
		"URL_LISTENPORT":     "9090",
		"URL_INSTANCEID":     "test-instance",
		"URL_RETRIES":        "5",
		"URL_LOGLEVEL":       "debug",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := &Config{
		Targets:       []string{"https://example.com", "https://test.com"},
		CheckInterval: 60 * time.Second,
		Timeout:       15 * time.Second,
		ListenPort:    9090,
		InstanceID:    "test-instance",
		Retries:       5,
		LogLevel:      "debug",
	}

	assertConfig(t, expected, cfg)
}

func TestLoad_WithTargetsEnvironmentTrimming(t *testing.T) {
	clearEnv(t)
	t.Setenv("URL_TARGETS", " https://example.com , https://test.com , https://another.com ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := []string{"https://example.com", "https://test.com", "https://another.com"}
	if len(cfg.Targets) != len(expected) {
		t.Fatalf("Expected %d targets, got %d", len(expected), len(cfg.Targets))
	}

	for i, target := range cfg.Targets {
		if target != expected[i] {
			t.Errorf("Target %d: expected %q, got %q", i, expected[i], target)
		}
	}
}

func TestLoad_WithConfigFile(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
targets:
  - "https://file-example.com"
  - "https://file-test.com"
checkInterval: 45s
timeout: 20s
listenPort: 7777
instanceId: "file-instance"
retries: 2
logLevel: "warn"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Setenv("URL_CONFIG_FILE", configPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := &Config{
		Targets:       []string{"https://file-example.com", "https://file-test.com"},
		CheckInterval: 45 * time.Second,
		Timeout:       20 * time.Second,
		ListenPort:    7777,
		InstanceID:    "file-instance",
		Retries:       2,
		LogLevel:      "warn",
	}

	assertConfig(t, expected, cfg)
}

func TestLoad_ConfigFileOverridesByEnvironment(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
targets:
  - "https://file-example.com"
checkInterval: 45s
timeout: 20s
listenPort: 7777
instanceId: "file-instance"
retries: 2
logLevel: "warn"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Setenv("URL_CONFIG_FILE", configPath)
	t.Setenv("URL_TARGETS", "https://env-override.com")
	t.Setenv("URL_LISTENPORT", "8888")
	t.Setenv("URL_LOGLEVEL", "error")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Environment should override file values
	if len(cfg.Targets) != 1 || cfg.Targets[0] != "https://env-override.com" {
		t.Errorf("Environment should override file targets, got: %v", cfg.Targets)
	}

	if cfg.ListenPort != 8888 {
		t.Errorf("Environment should override file port, got: %d", cfg.ListenPort)
	}

	if cfg.LogLevel != "error" {
		t.Errorf("Environment should override file log level, got: %s", cfg.LogLevel)
	}

	// Non-overridden values should come from file
	if cfg.CheckInterval != 45*time.Second {
		t.Errorf("Non-overridden values should come from file, got: %v", cfg.CheckInterval)
	}

	if cfg.InstanceID != "file-instance" {
		t.Errorf("Non-overridden instance ID should come from file, got: %v", cfg.InstanceID)
	}
}

func TestLoad_PartialEnvironmentOverrides(t *testing.T) {
	clearEnv(t)

	// Set only some environment variables
	t.Setenv("URL_CHECKINTERVAL", "120s")
	t.Setenv("URL_RETRIES", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Overridden values
	if cfg.CheckInterval != 120*time.Second {
		t.Errorf("CheckInterval should be overridden: expected %v, got %v", 120*time.Second, cfg.CheckInterval)
	}

	if cfg.Retries != 10 {
		t.Errorf("Retries should be overridden: expected %d, got %d", 10, cfg.Retries)
	}

	// Default values for non-overridden fields
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout should use default: expected %v, got %v", 10*time.Second, cfg.Timeout)
	}

	if cfg.ListenPort != 8412 {
		t.Errorf("ListenPort should use default: expected %d, got %d", 8412, cfg.ListenPort)
	}
}

func TestLoad_NoTargetsError(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
targets: []
checkInterval: 30s
timeout: 10s
listenPort: 8412
retries: 3
logLevel: "info"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Setenv("URL_CONFIG_FILE", configPath)

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error when no targets specified")
	}

	if !strings.Contains(err.Error(), "no targets specified") {
		t.Errorf("Expected 'no targets specified' error, got: %v", err)
	}
}

func TestLoad_EmptyTargetsEnvironmentUsesDefaults(t *testing.T) {
	clearEnv(t)

	t.Setenv("URL_TARGETS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed with empty URL_TARGETS, got: %v", err)
	}

	if len(cfg.Targets) != 2 {
		t.Errorf("Expected 2 default targets, got %d", len(cfg.Targets))
	}

	expectedTargets := []string{"https://google.com", "https://github.com"}
	for i, target := range expectedTargets {
		if i < len(cfg.Targets) && cfg.Targets[i] != target {
			t.Errorf("Target %d: expected %q, got %q", i, target, cfg.Targets[i])
		}
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	invalidConfigContent := `
targets:
  - "https://example.com"
checkInterval: invalid_duration
`

	if err := os.WriteFile(configPath, []byte(invalidConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Setenv("URL_CONFIG_FILE", configPath)

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error when config file has invalid content")
	}

	if !strings.Contains(err.Error(), "failed to load configuration") {
		t.Errorf("Expected configuration load error, got: %v", err)
	}
}

func TestLoad_NonexistentConfigFileFallsBackToDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("URL_CONFIG_FILE", "/nonexistent/path/config.yaml")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed and fall back to defaults, got: %v", err)
	}

	if len(cfg.Targets) != 2 {
		t.Errorf("Should fall back to default targets, got %d targets", len(cfg.Targets))
	}
}

func TestLoadConfigFile_EnvironmentPath(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	configContent := "targets:\n  - \"https://test.com\""
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Setenv("URL_CONFIG_FILE", configPath)

	content, err := loadConfigFile()
	if err != nil {
		t.Fatalf("loadConfigFile() failed: %v", err)
	}

	if !strings.Contains(content, "https://test.com") {
		t.Errorf("Expected config content to contain test URL, got: %s", content)
	}
}

func TestLoadConfigFile_StandardLocations(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	configContent := "targets:\n  - \"https://standard.com\""
	configPath := "./config.yaml"

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	content, err := loadConfigFile()
	if err != nil {
		t.Fatalf("loadConfigFile() failed: %v", err)
	}

	if !strings.Contains(content, "https://standard.com") {
		t.Errorf("Expected config content to contain standard URL, got: %s", content)
	}
}

func TestLoadConfigFile_NoConfigFound(t *testing.T) {
	clearEnv(t)

	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	_, err = loadConfigFile()
	if err == nil {
		t.Fatal("Expected error when no config file found")
	}

	if !strings.Contains(err.Error(), "no config file found") {
		t.Errorf("Expected 'no config file found' error, got: %v", err)
	}
}

func TestConfig_ValidateInstanceIDHostname(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.InstanceID == "" {
		t.Error("InstanceID should be set to hostname when not provided")
	}

	if len(cfg.InstanceID) < 1 {
		t.Error("Generated InstanceID should not be empty")
	}
}

func TestConfig_EnvironmentVariablePrecedence(t *testing.T) {
	clearEnv(t)

	tests := []struct {
		name     string
		envVar   string
		envValue string
		expected interface{}
		getValue func(*Config) interface{}
	}{
		{
			name:     "URL_CHECKINTERVAL overrides default",
			envVar:   "URL_CHECKINTERVAL",
			envValue: "90s",
			expected: 90 * time.Second,
			getValue: func(c *Config) interface{} { return c.CheckInterval },
		},
		{
			name:     "URL_TIMEOUT overrides default",
			envVar:   "URL_TIMEOUT",
			envValue: "25s",
			expected: 25 * time.Second,
			getValue: func(c *Config) interface{} { return c.Timeout },
		},
		{
			name:     "URL_LISTENPORT overrides default",
			envVar:   "URL_LISTENPORT",
			envValue: "9999",
			expected: 9999,
			getValue: func(c *Config) interface{} { return c.ListenPort },
		},
		{
			name:     "URL_RETRIES overrides default",
			envVar:   "URL_RETRIES",
			envValue: "7",
			expected: 7,
			getValue: func(c *Config) interface{} { return c.Retries },
		},
		{
			name:     "URL_LOGLEVEL overrides default",
			envVar:   "URL_LOGLEVEL",
			envValue: "error",
			expected: "error",
			getValue: func(c *Config) interface{} { return c.LogLevel },
		},
		{
			name:     "URL_INSTANCEID overrides default",
			envVar:   "URL_INSTANCEID",
			envValue: "custom-instance",
			expected: "custom-instance",
			getValue: func(c *Config) interface{} { return c.InstanceID },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(tt.envVar, tt.envValue)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			actual := tt.getValue(cfg)
			if actual != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestConfig_TargetsValidation(t *testing.T) {
	clearEnv(t)

	tests := []struct {
		name      string
		targets   string
		expectErr bool
	}{
		{
			name:      "valid single URL",
			targets:   "https://example.com",
			expectErr: false,
		},
		{
			name:      "valid multiple URLs",
			targets:   "https://example.com,https://test.com",
			expectErr: false,
		},
		{
			name:      "URLs with spaces",
			targets:   " https://example.com , https://test.com ",
			expectErr: false,
		},
		{
			name:      "empty targets should use defaults",
			targets:   "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			if tt.targets != "" {
				t.Setenv("URL_TARGETS", tt.targets)
			}

			cfg, err := Load()
			if tt.expectErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tt.expectErr && len(cfg.Targets) == 0 {
				t.Errorf("Expected targets to be populated")
			}
		})
	}
}

// Helper functions

func clearEnv(t *testing.T) {
	envVars := []string{
		"URL_TARGETS",
		"URL_CHECKINTERVAL",
		"URL_TIMEOUT",
		"URL_LISTENPORT",
		"URL_INSTANCEID",
		"URL_RETRIES",
		"URL_LOGLEVEL",
		"URL_CONFIG_FILE",
	}

	for _, env := range envVars {
		t.Setenv(env, "")
	}
}

func assertConfig(t *testing.T, expected, actual *Config) {
	t.Helper()

	if len(actual.Targets) != len(expected.Targets) {
		t.Errorf("Targets length: expected %d, got %d", len(expected.Targets), len(actual.Targets))
	}

	for i, target := range expected.Targets {
		if i < len(actual.Targets) && actual.Targets[i] != target {
			t.Errorf("Target %d: expected %q, got %q", i, target, actual.Targets[i])
		}
	}

	if actual.CheckInterval != expected.CheckInterval {
		t.Errorf("CheckInterval: expected %v, got %v", expected.CheckInterval, actual.CheckInterval)
	}

	if actual.Timeout != expected.Timeout {
		t.Errorf("Timeout: expected %v, got %v", expected.Timeout, actual.Timeout)
	}

	if actual.ListenPort != expected.ListenPort {
		t.Errorf("ListenPort: expected %d, got %d", expected.ListenPort, actual.ListenPort)
	}

	if expected.InstanceID != "" && actual.InstanceID != expected.InstanceID {
		t.Errorf("InstanceID: expected %q, got %q", expected.InstanceID, actual.InstanceID)
	}

	if actual.Retries != expected.Retries {
		t.Errorf("Retries: expected %d, got %d", expected.Retries, actual.Retries)
	}

	if actual.LogLevel != expected.LogLevel {
		t.Errorf("LogLevel: expected %q, got %q", expected.LogLevel, actual.LogLevel)
	}
}