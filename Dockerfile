# Build stage
FROM golang:1.25-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o replicator cmd/replicator/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S replicator && \
    adduser -u 1001 -S replicator -G replicator

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/replicator .

# Create directories and set permissions
RUN mkdir -p /app/config /app/positions && \
    chown -R replicator:replicator /app

# Switch to non-root user
USER replicator

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command - use environment variable for config file
ENTRYPOINT ["./replicator"]
CMD [ "--config", "/app/config/config.yaml" ]