# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates and netcat for health check
RUN apk --no-cache add ca-certificates tzdata nc

# Copy binary from builder
COPY --from=builder /app/server .
COPY --from=builder /app/config.yaml.example ./config.yaml

# Create non-root user
RUN adduser -D -u 1000 appuser
USER appuser

# Expose port
EXPOSE 8080

# Health check using TCP port check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD nc -z localhost 8080 || exit 1

# Run the application
ENTRYPOINT ["./server"]
