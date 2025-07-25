package config

import (
	_ "embed"
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
	CheckInterval time.Duration `yaml:"checkInterval"`
	Timeout       time.Duration `yaml:"timeout"`
	ListenPort    int           `yaml:"listenPort"`
	InstanceID    string        `yaml:"instanceId"`
	Retries       int           `yaml:"retries"`
	LogLevel      string        `yaml:"logLevel"`
}

//go:embed config.default.yml
var defaultYAML string

// Load loads configuration using jasoet/pkg/config patterns
func Load() (*Config, error) {
	configContent, err := loadConfigFile()
	if err != nil {
		configContent = defaultYAML
	}

	cfg, err := config.LoadStringWithConfig[Config](configContent, func(v *viper.Viper) {
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

	if cfg.InstanceID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		cfg.InstanceID = hostname
	}

	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("no targets specified")
	}

	return cfg, nil
}

// loadConfigFile attempts to load configuration from standard locations
func loadConfigFile() (string, error) {
	if configPath := os.Getenv("URL_CONFIG_FILE"); configPath != "" {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
		return string(content), nil
	}

	configPaths := []string{
		"./config.yaml",
		"./configs/config.yaml",
		"/etc/url-exporter/config.yaml",
	}
	
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