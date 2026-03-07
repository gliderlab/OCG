# ==========================================
# Stage 1: Build llama-server (C++)
# ==========================================
FROM alpine:3.19 AS cpp-builder

RUN apk add --no-cache \
    build-base \
    cmake \
    git \
    linux-headers

WORKDIR /src
# Clone llama.cpp source code to build the server
RUN git clone https://github.com/ggml-org/llama.cpp.git && \
    cd llama.cpp

WORKDIR /src/llama.cpp/build
RUN cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DLLAMA_BUILD_SERVER=ON \
    -DLLAMA_BUILD_EXAMPLES=OFF \
    -DLLAMA_BUILD_TESTS=OFF \
    -DLLAMA_BUILD_EMBEDDING=ON \
    -DBUILD_SHARED_LIBS=OFF && \
    cmake --build . --config Release --target llama-server -j $(nproc)

# ==========================================
# Stage 2: Build OCG Go binaries
# ==========================================
FROM golang:1.24-alpine AS go-builder

# Install CGO dependencies required for go-sqlite3
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build OCG process manager
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/ocg ./cmd/ocg
# Build OCG Gateway
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/ocg-gateway ./cmd/gateway
# Build OCG Agent (with SQLite FTS5 support)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -tags "sqlite_fts5" -o bin/ocg-agent ./cmd/agent
# Build OCG Embedding server
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/ocg-embedding ./cmd/embedding-server

# ==========================================
# Stage 3: Final Runtime Image
# ==========================================
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite libstdc++ libgomp

WORKDIR /app

# Copy all compiled binaries from the builder stages
COPY --from=go-builder /app/bin/ /app/
COPY --from=cpp-builder /src/llama.cpp/build/bin/llama-server /app/

# Create required directories
RUN mkdir -p /app/data /app/config /app/models

# Set environment variables for runtime configuration
ENV OCG_DATA_DIR=/app/data
ENV OCG_HOST=0.0.0.0
ENV OCG_PORT=8080
ENV LLAMA_SERVER_BIN=/app/llama-server

# Persist data directory
VOLUME ["/app/data"]

EXPOSE 8080

# Start the OCG process manager in foreground mode to keep container alive
ENTRYPOINT ["/app/ocg", "start", "--foreground"]
