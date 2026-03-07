# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
# Note: sqlite requires CGO, so we need gcc and musl-dev
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o ocg ./cmd/ocg

# Final stage
FROM alpine:3.19

# Install runtime dependencies (ca-certificates for HTTPS, tzdata for cron timezone)
RUN apk add --no-cache ca-certificates tzdata sqlite

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/ocg /app/

# Create directories for data and config
RUN mkdir -p /app/data /app/config

# Set environment variables
ENV OCG_DATA_DIR=/app/data
ENV OCG_HOST=0.0.0.0
ENV OCG_PORT=8080

# Volumes for persistent data
VOLUME ["/app/data"]

# Expose the API and UI port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/app/ocg"]
