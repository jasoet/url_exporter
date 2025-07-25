package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jasoet/pkg/config"
	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Targets       []string      `yaml:"targets"`
	CheckInterval time.Duration `yaml:"check_interval"`
	Timeout       time.Duration `yaml:"timeout"`
	ListenPort    int           `yaml:"listen_port"`
	InstanceID    string        `yaml:"instance_id"`
	Retries       int           `yaml:"retries"`
	LogLevel      string        `yaml:"log_level"`
}

// DefaultYAML provides default configuration in YAML format
const DefaultYAML = `targets:
  - "https://example.com"
check_interval: 30s
timeout: 10s
listen_port: 8080
instance_id: ""
retries: 3
log_level: "info"`

// Load loads configuration using jasoet/pkg/config patterns
func Load() (*Config, error) {
	// Try to load from file first
	configContent, err := loadConfigFile()
	if err != nil {
		// Fallback to default if no config file found
		configContent = DefaultYAML
	}

	// Load configuration with environment variable override
	cfg, err := config.LoadStringWithConfig[Config](configContent, func(v *viper.Viper) {
		// Handle comma-separated targets from environment
		if targetsEnv := os.Getenv("URL_TARGETS"); targetsEnv != "" {
			targets := strings.Split(targetsEnv, ",")
			for i := range targets {
				targets[i] = strings.TrimSpace(targets[i])
			}
			v.Set("targets", targets)
		}
	}, "URL")

	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set instance ID if not provided
	if cfg.InstanceID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		cfg.InstanceID = hostname
	}

	// Validate we have at least one target
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("no targets specified")
	}

	return cfg, nil
}

// loadConfigFile attempts to load configuration from standard locations
func loadConfigFile() (string, error) {
	// Check for config file path from environment
	if configPath := os.Getenv("URL_CONFIG_FILE"); configPath != "" {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
		return string(content), nil
	}

	// Search in standard locations
	configPaths := []string{
		"./config.yaml",
		"./configs/config.yaml",
		"/etc/url-exporter/config.yaml",
	}
	
	// Add home directory path
	if homeDir, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths, homeDir+"/.url-exporter/config.yaml")
	}

	for _, path := range configPaths {
		if content, err := os.ReadFile(path); err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("no config file found")
}