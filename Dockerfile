FROM ubuntu:24.04

# Update and install runtime dependencies, then clean up to reduce image size
RUN apt-get update && \
    apt-get install -y ca-certificates tzdata curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd --create-home --shell /bin/bash --user-group --uid 1000 appuser

WORKDIR /app

# Copy the pre-built binary (goreleaser will handle this)
COPY url-exporter .

# Make binary executable and change ownership to non-root user
RUN chmod +x /app/url-exporter && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose metrics port
EXPOSE 8412

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8412/health || exit 1

# Set entrypoint
ENTRYPOINT ["/app/url-exporter"]