# Prometheus URL Availability Exporter

A Go-based Prometheus exporter that monitors URL availability and exposes metrics for infrastructure health monitoring across multiple network locations.

## Features

- **Concurrent URL Checking**: Uses worker pools for efficient concurrent monitoring
- **Comprehensive Metrics**: Tracks availability, response times, status codes, and check counts
- **Flexible Configuration**: Supports YAML files, environment variables, and command-line flags
- **Multi-location Monitoring**: Instance labeling for identifying different network locations
- **Retry Logic**: Configurable retry attempts for failed requests
- **Production Ready**: Includes Docker, Kubernetes, and systemd deployment configurations

## Quick Start

### Using Pre-built Binary

1. Download the latest release or build from source:
```bash
# Using Task (recommended)
task build

# Or manually
go build -o dist/url-exporter ./app
```

2. Run the exporter (uses default config locations or environment variables):
```bash
# Option 1: Use environment variables
URL_TARGETS="https://example.com,https://google.com" ./dist/url-exporter

# Option 2: Create config file in standard location
cp configs/config.example.yaml ./config.yaml
# Edit config.yaml with your URLs, then run:
./dist/url-exporter
```

4. View metrics:
```bash
curl http://localhost:8412/metrics
```

### Using Docker

```bash
# Build the image
docker build -t url-exporter:latest .

# Run with environment variables
docker run -p 8412:8412 \
  -e URL_TARGETS="https://example.com,https://google.com" \
  url-exporter:latest
```

### Using Taskfile.dev (Recommended)

```bash
# Initial project setup (installs tools and dependencies)
task setup

# Available tasks:
task build           # Build the application
task test            # Run all tests with coverage
task lint            # Run code quality checks
task run             # Run with example config
task docker-build    # Build Docker image
task docker-run      # Run in Docker container
task clean           # Clean build artifacts

# Release tasks
task release-snapshot    # Create snapshot release (for testing)
task release            # Create release (CI/CD only)

# Full development cycle
task dev             # install-tools + deps + quality + test + build

# CI/CD simulation
task ci              # Complete CI pipeline
```

## Configuration

### Configuration File (YAML)

```yaml
targets:
  - "https://example.com"
  - "https://api.service.com/health"
  - "http://internal-service:8412/status"
checkInterval: 30s      # Changed from check_interval
timeout: 10s
listenPort: 8412        # Changed from listen_port
instanceId: "vm-prod-us-east"  # Changed from instance_id (Optional)
retries: 3
logLevel: "info"        # Changed from log_level
```

### Environment Variables

```bash
# URLs to monitor (comma-separated)
export URL_TARGETS="https://example.com,https://api.service.com/health"

# Configuration options (note: camelCase in YAML, but env vars use original names)
export URL_CHECKINTERVAL="30s"    # Maps to checkInterval in YAML
export URL_TIMEOUT="10s"
export URL_LISTENPORT="8412"      # Maps to listenPort in YAML
export URL_INSTANCEID="vm-prod-01"  # Maps to instanceId in YAML
export URL_RETRIES="3"
export URL_LOGLEVEL="info"        # Maps to logLevel in YAML
```

### Configuration File Locations

The application searches for configuration files in this order:

**Priority 1:** Environment variable (if set)
```bash
export URL_CONFIG_FILE="/path/to/your/config.yaml"
```

**Priority 2:** Standard locations (searched in order):
1. `./config.yaml` (current directory)
2. `~/.url-exporter/config.yaml` (user home directory)

If no config file is found, the application falls back to embedded defaults.

### Configuration Priority

1. Environment variables (highest priority)
2. Configuration file (from locations above)
3. Default values (lowest priority)

## Metrics

The exporter provides the following Prometheus metrics:

### Primary Metrics

- **`url_up{url, host, path, instance}`** - Binary metric (1 if URL returns 2xx status, 0 otherwise)
- **`url_response_time_milliseconds{url, host, path, instance}`** - Response time in milliseconds
- **`url_http_status_code{url, host, path, instance}`** - HTTP status code returned
- **`url_check_total{url, host, path, status_code, instance}`** - Total checks counter by status
- **`url_status_code_total{url, host, path, status_code, instance}`** - Counter per status code

### Label Structure

For URL `https://api.service.com/health`:
- `url`: `"https://api.service.com/health"` (complete URL)
- `host`: `"https://api.service.com"` (scheme + hostname)
- `path`: `"/health"` (path component)
- `instance`: `"vm-prod-01"` (VM hostname or custom identifier)

## Endpoints

- **`/metrics`** - Prometheus metrics endpoint
- **`/health`** - Health check endpoint
- **`/`** - Service information and status

## Deployment

### Docker

```bash
# Build image
docker build -t url-exporter:latest .

# Run container with config file mounted to standard location
docker run -d \
  --name url-exporter \
  -p 8412:8412 \
  -v $(pwd)/configs/config.yaml:/app/config.yaml \
  url-exporter:latest
```

### Docker Compose (Optional)

Create `docker-compose.yml`:
```yaml
version: '3.8'
services:
  url-exporter:
    build: .
    ports:
      - "8412:8412"
    environment:
      - URL_TARGETS=https://google.com,https://github.com
      - URL_LOG_LEVEL=info
      - URL_CHECK_INTERVAL=30s
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8412/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'url-exporter'
    static_configs:
      - targets: ['localhost:8412']
    scrape_interval: 30s
    metrics_path: /metrics
```

## Development

### Prerequisites

- Go 1.21 or later
- [Taskfile.dev](https://taskfile.dev/) (required for build automation)
- Docker (for containerization)

### Initial Setup

```bash
# Install development tools and dependencies
task setup
```

This will install:
- goreleaser (for cross-platform builds)
- golangci-lint (for code quality)
- Download Go module dependencies

### Building

```bash
# Single platform build (recommended for development)
task build

# Multi-platform build using goreleaser
task build-all

# Create release snapshot (includes Docker image)
task release-snapshot
```

### Testing

```bash
# Run tests with coverage (generates coverage files in dist/)
task test

# Or manually
go test -race -coverprofile=dist/coverage.out -v ./...
go tool cover -html=dist/coverage.out -o dist/coverage.html
```

### Code Quality

```bash
# Run all quality checks
task quality

# Individual checks
task fmt     # Format code
task vet     # Run go vet  
task lint    # Run golangci-lint

# Or manually
gofmt -s -w .
go vet ./...
golangci-lint run
```

## Architecture

The application follows the jasoet/pkg patterns for production-ready Go applications:

1. **Configuration Management** (`internal/config/`)
   - Uses `config.LoadString[T]` pattern from jasoet/pkg/config
   - Type-safe configuration with automatic environment variable override
   - Supports YAML files with ENV variable precedence

2. **URL Checker** (`internal/checker/`)
   - Uses `concurrent.ExecuteConcurrently` pattern from jasoet/pkg/concurrent
   - Type-safe concurrent execution without raw goroutines
   - Implements retry logic and error handling

3. **Metrics Collector** (`internal/metrics/`)
   - Implements Prometheus collector interface
   - Manages metric registration and updates
   - Processes check results and maintains counters

4. **HTTP Server** (`internal/server/`)
   - Uses `server.Start()` function from jasoet/pkg/server
   - Production-ready Echo server with built-in health checks
   - Automatic graceful shutdown handling

## Troubleshooting

### Common Issues

1. **Connection refused errors**
   ```bash
   # Check if target URL is reachable
   curl -I https://example.com
   ```

2. **SSL certificate errors**
   ```bash
   # For internal services, you might need to skip SSL verification
   # This should be configured in the checker if needed
   ```

3. **High memory usage**
   ```bash
   # Reduce check interval or number of targets
   # Monitor with: curl http://localhost:8412/metrics | grep go_
   ```

### Logs

Set log level to debug for detailed information:

```bash
export URL_LOGLEVEL=debug
./dist/url-exporter
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `task ci` to verify everything passes
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:
- Create an issue on GitHub
- Check the logs with debug level enabled
- Verify configuration syntax

## Related Projects

- [Prometheus](https://prometheus.io/) - Monitoring and alerting toolkit
- [Blackbox Exporter](https://github.com/prometheus/blackbox_exporter) - Generic probe exporter
- [jasoet/pkg](https://github.com/jasoet/pkg) - Reusable Go components used in this project