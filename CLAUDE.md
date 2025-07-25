# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project: Prometheus URL Availability Exporter

A Go-based Prometheus exporter that monitors URL availability and exposes metrics for infrastructure health monitoring across multiple network locations.

## Development Commands

### Using Taskfile.dev (Required)
```bash
# Initial project setup (installs tools and dependencies)
task setup           # Install goreleaser, golangci-lint, and download dependencies

# Development tasks
task build           # Build the application to dist/
task test            # Run all tests with coverage (outputs to dist/)
task lint            # Run code quality checks
task run             # Run with example config
task docker-build    # Build Docker image
task docker-run      # Run in Docker container
task clean           # Clean build artifacts

# Release tasks  
task release-snapshot    # Create snapshot release with all platforms + Docker
task release            # Create release (CI/CD only, requires GitHub env vars)

# Composite tasks
task dev             # Full development cycle: install-tools + deps + quality + test + build
task ci              # CI/CD pipeline simulation
```

### Manual Commands (if Taskfile not available)
```bash
# Tool Installation
go install github.com/goreleaser/goreleaser/v2@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Building
go mod download && go build -o dist/url-exporter .

# Testing (with coverage in dist/)
go test -race -coverprofile=dist/coverage.out -v ./...
go tool cover -html=dist/coverage.out -o dist/coverage.html

# Running
# Option 1: With environment variables
URL_TARGETS="https://example.com,https://google.com" ./dist/url-exporter

# Option 2: With config file (copy to standard location first)
cp configs/config.example.yaml ./config.yaml
./dist/url-exporter

# Code Quality
gofmt -s -w . && go vet ./... && golangci-lint run

# Release (snapshot)
GITHUB_REPOSITORY_OWNER=jasoet GITHUB_REPOSITORY_NAME=url_exporter IMAGE_NAME=url-exporter GITHUB_REPOSITORY=jasoet/url_exporter goreleaser release --snapshot --clean
```

## Architecture Overview

### Core Components

1. **Configuration Management** (`internal/config/`)
   - **CRITICAL**: Uses `config.LoadString[T]` pattern from jasoet/pkg/config examples
   - **DO NOT** implement Viper directly - use the established patterns
   - Supports YAML files, environment variables with automatic override
   - Priority: ENV vars > Config file values

2. **URL Checker** (`internal/checker/`)
   - **CRITICAL**: Uses `concurrent.ExecuteConcurrently` pattern from jasoet/pkg/concurrent examples
   - **DO NOT** use raw goroutines - use the established concurrent execution patterns
   - Performs HTTP HEAD requests with configurable timeouts
   - Implements retry logic and error handling

3. **Metrics Collector** (`internal/metrics/`)
   - Implements Prometheus collector interface
   - Exposes 6 comprehensive metrics: 4 gauges + 2 counters
   - All metrics include proper labels for multi-dimensional monitoring
   - Manages metric registration, updates, and counter tracking

4. **HTTP Server** (`internal/server/`)
   - **CRITICAL**: Uses `server.Start()` function pattern from jasoet/pkg/server examples
   - **DO NOT** setup Echo directly - use the established server patterns
   - Exposes /metrics (Prometheus), /health endpoints
   - Built-in graceful shutdown

### External Dependencies
- `github.com/prometheus/client_golang` - Prometheus metrics
- `github.com/jasoet/pkg/config` - Configuration with Viper
- `github.com/jasoet/pkg/concurrent` - Concurrent execution patterns
- `github.com/jasoet/pkg/server` - HTTP server with Echo

### Key Design Patterns
- **config.LoadString[T]** pattern for type-safe configuration with env override
- **concurrent.ExecuteConcurrently** pattern for type-safe concurrent operations
- **server.Start()** pattern for production-ready HTTP server
- Collector pattern for Prometheus metrics
- Context-based operations with proper cancellation

### Metrics Details

#### Fully Implemented (6 metrics)

**Gauge Metrics** (labels: `url`, `host`, `path`, `instance`):
- `url_up` - 1 if URL returns 2xx status, 0 otherwise
- `url_error` - 1 if network/connection error occurred, 0 otherwise  
- `url_response_time_milliseconds` - Response time (only when no error)
- `url_http_status_code` - HTTP status code (only when no error)

**Counter Metrics** (labels: `url`, `host`, `path`, `status_code`, `instance`):
- `url_check_total` - Total number of checks performed by status code
- `url_status_code_total` - Counter for each specific HTTP status code encountered

All metrics are properly implemented and exposed via the `/metrics` endpoint.

## Project Structure
```
url-exporter/
├── main.go                 # Application entry point
├── main_test.go           # Main package tests
├── internal/              # Private application code
│   ├── config/           # Configuration structures and loading
│   ├── checker/          # URL checking logic
│   ├── metrics/          # Prometheus metrics implementation
│   └── server/           # HTTP server setup
├── configs/              # Example configuration files
├── Dockerfile            # Docker container configuration
└── docs/                 # Project documentation
```

## Important Notes

- **CRITICAL**: Must follow jasoet/pkg example patterns exactly - DO NOT implement from scratch
- Always use header-only requests (no body download) for efficiency
- Instance identification is critical for multi-location monitoring
- All 6 metrics must include proper labels: 4 gauges with (url, host, path, instance), 2 counters with additional status_code label
- Configuration validation happens at startup
- Use Taskfile.dev for build system (NOT Makefile)
- Project specification is in docs/SPECIFICATION.md