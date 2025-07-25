FROM ubuntu:latest

# Update and install ca-certificates, then clean up to reduce image size
RUN apt-get update && \
    apt-get install -y ca-certificates curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY url-exporter .

RUN chmod +x /app/url-exporter

EXPOSE 8412

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8412/health || exit 1

ENTRYPOINT ["/app/url-exporter"]