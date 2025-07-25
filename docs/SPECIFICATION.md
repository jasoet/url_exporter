# Prometheus URL Availability Exporter - Project Specification (Updated)

## Overview
A Go-based Prometheus exporter that monitors URL availability and exposes metrics for monitoring infrastructure health across multiple network locations.

## Project Structure
- Built with Go using the official Prometheus client library
- **CRITICAL**: Must follow exact patterns from https://github.com/jasoet/pkg examples
- **DO NOT implement from raw libraries** (Viper, Echo, etc.)
- **MUST use** LoadString patterns for config, ExecuteConcurrently for concurrent operations, Start function for server
- Follows Prometheus exporter best practices

## Core Functionality
The exporter performs the following operations:
1. Accepts a configurable list of URLs to monitor
2. Periodically checks each URL's availability (default: 30-second intervals)
3. For each check:
   - Performs HTTP GET request (headers only, no body download)
   - Records response time and HTTP status code
   - Handles timeouts and errors gracefully

## Metrics Exposed

### Primary Metrics
- `url_up{url, host, path, instance}` - Binary metric (1=reachable with 2xx, 0=otherwise)
- `url_response_time_seconds{url, host, path, instance}` - Response time in seconds
- `url_http_status_code{url, host, path, instance}` - HTTP status code returned
- `url_check_total{url, host, path, status_code, instance}` - Total checks counter by status
- `url_status_code_total{url, host, path, status_code, instance}` - Counter per status code

### Label Structure
For URL "https://api.service.com/health":
- `url`: "https://api.service.com/health" (complete URL)
- `host`: "https://api.service.com" (scheme + hostname)
- `path`: "/health" (path component)
- `instance`: "vm-prod-01" (VM hostname or custom identifier)

## Configuration

### YAML Configuration File
```yaml
targets:
  - "https://example.com"
  - "https://api.service.com/health"
  - "http://internal-service:8080/status"
check_interval: 30s
timeout: 10s
listen_port: 8080
instance_id: "vm-prod-us-east"  # Optional
retries: 3
log_level: "info"
```

### Environment Variables
```bash
URL_TARGETS="https://example.com,https://api.service.com/health"
URL_CHECK_INTERVAL="30s"
URL_TIMEOUT="10s"
URL_LISTEN_PORT="8080"
URL_INSTANCE_ID="vm-prod-us-east"
URL_RETRIES="3"
URL_LOG_LEVEL="info"
URL_CONFIG_FILE="/etc/url-exporter/config.yaml"
```

### Command Line Flags
```bash
./url-exporter --config=config.yaml --port=8080 --instance-id=vm-01
```

### Configuration Priority
1. Command line flags (highest)
2. Environment variables
3. Configuration file
4. Default values (lowest)

### Configuration File Search Paths
1. `./config.yaml`
2. `/etc/url-exporter/config.yaml`
3. `$HOME/.url-exporter/config.yaml`

## Technical Implementation

### Dependencies
- `github.com/prometheus/client_golang` - Prometheus metrics
- `github.com/jasoet/pkg/config` - Configuration management with Viper
- `github.com/jasoet/pkg/concurrent` - Concurrent URL checking
- `github.com/jasoet/pkg/server` - HTTP server with Echo framework

### Key Features
- Concurrent URL checking with worker pools
- Graceful shutdown handling
- Automatic retry logic for failed requests
- Comprehensive error handling (DNS, connection, SSL/TLS)
- Support for both HTTP and HTTPS
- Auto-detection of VM hostname/IP for instance labeling
- Efficient header-only HTTP requests

### Endpoints
- `/metrics` - Prometheus metrics endpoint
- `/health` - Health check endpoint
- `/` - Basic status page

## Error Handling
- Network timeouts with configurable duration
- DNS resolution failures
- Connection refused errors
- SSL/TLS certificate validation
- HTTP error status codes
- Configurable retry logic

## Deployment

### Docker
- Multi-stage Dockerfile for minimal image size
- Support for configuration via environment variables
- Health check included

### Systemd
- Service file for system deployment
- Automatic restart on failure
- Proper logging integration

### Kubernetes
- Example deployment manifest
- ConfigMap for configuration
- Service for metrics exposure

## Monitoring Integration

### Prometheus Configuration
```yaml
scrape_configs:
  - job_name: 'url-exporter'
    static_configs:
      - targets: ['localhost:8080']
```

### Grafana Dashboard
- Example dashboard JSON included
- Pre-configured panels for all metrics
- Alert rule examples

## Development Workflow

### Using Taskfile.dev (NOT Makefile)
```bash
# Install Taskfile.dev first: https://taskfile.dev/
task build      # Build the application
task test       # Run tests
task lint       # Run linters
task run        # Run locally
task docker-build  # Build Docker image
task docker-run    # Run in Docker
task clean      # Clean build artifacts
```

### Manual Commands (if Taskfile not available)
```bash
go mod download
go build -o url-exporter ./cmd
go test ./...
```

## Project Structure
```
url-exporter/
├── cmd/
│   └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── checker/
│   │   └── checker.go
│   ├── metrics/
│   │   └── collector.go
│   └── server/
│       └── server.go
├── deployments/
│   ├── docker/
│   │   └── Dockerfile
│   ├── systemd/
│   │   └── url-exporter.service
│   └── kubernetes/
│       ├── deployment.yaml
│       └── configmap.yaml
├── configs/
│   └── config.example.yaml
├── docs/
│   └── SPECIFICATION.md
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── CLAUDE.md
```