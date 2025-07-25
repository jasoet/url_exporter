package config

import (
	_ "embed"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jasoet/pkg/config"
	"github.com/rs/zerolog/log"
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
			if ip, ipErr := getMachineIP(); ipErr == nil {
				cfg.InstanceID = ip
			} else {
				return nil, fmt.Errorf("failed to get hostname and IP address: hostname_err=%w, ip_err=%v", err, ipErr)
			}
		} else {
			cfg.InstanceID = hostname
		}
	}

	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("no targets specified")
	}

	return cfg, nil
}

func loadConfigFile() (string, error) {
	if configPath := os.Getenv("URL_CONFIG_FILE"); configPath != "" {
		log.Debug().
			Str("configPath", configPath).
			Msg("URL_CONFIG_FILE exist, read from it")

		content, err := os.ReadFile(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
		return string(content), nil
	}

	configPaths := []string{
		"./config.yaml",
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

func getMachineIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return getFirstNonLoopbackIP()
	}
	defer func() {
		_ = conn.Close()
	}()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func getFirstNonLoopbackIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					return ipNet.IP.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no non-loopback IP address found")
}
