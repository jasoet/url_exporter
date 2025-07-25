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
go build -o url-exporter ./cmd/url-exporter
```

2. Create a configuration file:
```bash
cp configs/config.example.yaml config.yaml
# Edit config.yaml with your URLs
```

3. Run the exporter:
```bash
./url-exporter --config=config.yaml
```

4. View metrics:
```bash
curl http://localhost:8080/metrics
```

### Using Docker

```bash
# Build the image
docker build -t url-exporter:latest .

# Run with environment variables
docker run -p 8080:8080 \
  -e URL_TARGETS="https://example.com,https://google.com" \
  url-exporter:latest
```

### Using Taskfile.dev (Recommended)

```bash
# Install Taskfile.dev first: https://taskfile.dev/
# Then run tasks:

task build           # Build the application
task test            # Run all tests
task test-race       # Run tests with race detection
task lint            # Run code quality checks
task run             # Run with example config
task docker-build    # Build Docker image
task docker-run      # Run in Docker container
task clean           # Clean build artifacts

# Full development cycle
task dev             # deps + quality + test + build

# CI/CD simulation
task ci              # Complete CI pipeline
```

## Configuration

### Configuration File (YAML)

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
# URLs to monitor (comma-separated)
export URL_TARGETS="https://example.com,https://api.service.com/health"

# Configuration options
export URL_CHECK_INTERVAL="30s"
export URL_TIMEOUT="10s"
export URL_LISTEN_PORT="8080"
export URL_INSTANCE_ID="vm-prod-01"
export URL_RETRIES="3"
export URL_LOG_LEVEL="info"
```

### Command Line Flags

```bash
./url-exporter \
  --targets="https://example.com,https://api.service.com/health" \
  --port=8080 \
  --instance-id="vm-01" \
  --check-interval="30s" \
  --timeout="10s" \
  --retries=3 \
  --log-level="info"
```

### Configuration Priority

1. Command line flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

## Metrics

The exporter provides the following Prometheus metrics:

### Primary Metrics

- **`url_up{url, host, path, instance}`** - Binary metric (1 if URL returns 2xx status, 0 otherwise)
- **`url_response_time_seconds{url, host, path, instance}`** - Response time in seconds
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

# Run container
docker run -d \
  --name url-exporter \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config.yaml \
  url-exporter:latest --config=/config.yaml
```

### Docker Compose (Optional)

Create `docker-compose.yml`:
```yaml
version: '3.8'
services:
  url-exporter:
    build: .
    ports:
      - "8080:8080"
    environment:
      - URL_TARGETS=https://google.com,https://github.com
      - URL_LOG_LEVEL=info
      - URL_CHECK_INTERVAL=30s
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
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
      - targets: ['localhost:8080']
    scrape_interval: 30s
    metrics_path: /metrics
```

## Development

### Prerequisites

- Go 1.21 or later
- [Taskfile.dev](https://taskfile.dev/) (recommended for build automation)
- Docker (for containerization)

### Building

```bash
# With Taskfile.dev (recommended)
task build

# Or manually
go mod download
go build -o bin/url-exporter ./app
```

### Testing

```bash
# With Taskfile.dev (recommended)
task test
task test-race
task test-coverage

# Or manually
go test ./...
go test -race ./...
```

### Code Quality

```bash
# With Taskfile.dev (recommended)
task fmt
task vet  
task lint
task quality  # runs all quality checks

# Or manually
go fmt ./...
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
   # Monitor with: curl http://localhost:8080/metrics | grep go_
   ```

### Logs

Set log level to debug for detailed information:

```bash
export URL_LOG_LEVEL=debug
./url-exporter --config=config.yaml
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
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