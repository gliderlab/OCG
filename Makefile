# OCG (OpenClaw-Go) Makefile
# Supports FAISS HNSW vector memory by default

.PHONY: all build build-no-faiss build-agent build-gateway build-embedding build-ocg clean run help test lint

BIN_DIR := bin

# Static build is optional. Set STATIC=1 to enable.
STATIC ?= 0
BASE_LDFLAGS := -s -w -buildid=

ifeq ($(STATIC),1)
LDFLAGS := $(BASE_LDFLAGS) -extldflags "-static"
FAISS_LDFLAGS := -Wl,-Bstatic -lfaiss -lgomp -lblas -llapack -lgfortran -lquadmath -Wl,-Bdynamic
else
LDFLAGS := $(BASE_LDFLAGS)
FAISS_LDFLAGS := -lfaiss -lgomp -lblas -llapack
endif

# Default target (FAISS HNSW on): build gateway + agent + embedding + ocg
all: build
	@echo "✅ Build complete!"
	@echo "   Binaries in $(BIN_DIR)/:"
	@echo "   - ocg            # Process manager (start/stop/status)"
	@echo "   - ocg-gateway   # Gateway HTTP server"
	@echo "   - ocg-agent     # Agent with FAISS HNSW"
	@echo "   - ocg-embedding # Local embedding service"
	@echo "   - llama-server  # llama.cpp server (optional)"

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# ==================== Build Targets ====================

# OCG process manager
build-ocg: $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ocg ./cmd/ocg/

# Gateway
build-gateway: $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ocg-gateway ./cmd/gateway/

# Agent (FAISS HNSW enabled by default)
build-agent: $(BIN_DIR)
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" \
	CGO_CXXFLAGS="-I/usr/include" \
	CGO_LDFLAGS="$(FAISS_LDFLAGS)" \
	go build -ldflags="$(LDFLAGS)" -tags "faiss sqlite_fts5" -o $(BIN_DIR)/ocg-agent ./cmd/agent/

# Agent without FAISS (fallback to SQLite linear search)
build-no-faiss: $(BIN_DIR)
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" \
	go build -ldflags="$(LDFLAGS)" -tags "sqlite_fts5" -o $(BIN_DIR)/ocg-agent ./cmd/agent/

# Embedding server
build-embedding: $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ocg-embedding ./cmd/embedding-server/

# Default build (FAISS on): ocg + gateway + agent + embedding
build: build-ocg build-gateway build-agent build-embedding
	@echo "✅ Build complete!"

# ==================== Special Builds ====================

# Build with binddb (SQLite binary binding enabled)
build-binddb: build-binddb-agent build-binddb-gateway
	@echo "✅ binddb build complete"

build-binddb-agent: $(BIN_DIR)
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" \
	CGO_CXXFLAGS="-I/usr/include" \
	CGO_LDFLAGS="$(FAISS_LDFLAGS)" \
	go build -ldflags="$(LDFLAGS)" -tags "faiss sqlite_fts5 binddb" -o $(BIN_DIR)/ocg-agent ./cmd/agent/

build-binddb-gateway: $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -tags "binddb" -o $(BIN_DIR)/ocg-gateway ./cmd/gateway/

# Lite build: no FAISS, SQLite only
build-lite: $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ocg ./cmd/ocg/
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ocg-gateway ./cmd/gateway/
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" \
	go build -ldflags="$(LDFLAGS)" -tags "sqlite_fts5" -o $(BIN_DIR)/ocg-agent ./cmd/agent/
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ocg-embedding ./cmd/embedding-server/

# llama.cpp server (requires llama.cpp source)
LLAMA_JOBS ?= 1

build-llama:
	$(MAKE) -C llama.cpp llama-server JOBS=$(LLAMA_JOBS)

# Build everything: gateway + agent + embedding + llama-server
build-all: build build-llama

# ==================== Testing & Quality ====================

# Run tests
test:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test -tags "sqlite_fts5" ./...

# Run tests with coverage
test-cover:
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test -tags "sqlite_fts5" -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Run linter
lint:
	golangci-lint run ./...
	@echo "✅ lint passed"

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Vet code
vet:
	go vet ./...

# ==================== Cleanup ====================

clean:
	rm -f $(BIN_DIR)/ocg $(BIN_DIR)/ocg-gateway $(BIN_DIR)/ocg-agent $(BIN_DIR)/ocg-embedding
	rm -f *.db *.log
	rm -f coverage.out

# Clean all including node_modules
clean-all: clean
	rm -rf gateway/static-vue/node_modules

# ==================== Help ====================

help:
	@echo "OCG (OpenClaw-Go) Build System"
	@echo ""
	@echo "📦 Standard Builds:"
	@echo "  make               # Default: Gateway + Agent (FAISS) + Embedding"
	@echo "  make build-no-faiss # Without FAISS, SQLite fallback"
	@echo "  make build-lite    # Lite: Gateway + Agent + Embedding (no FAISS)"
	@echo ""
	@echo "🔧 Special Builds:"
	@echo "  make build-binddb  # With SQLite binary binding"
	@echo "  make build-llama   # Build llama.cpp server"
	@echo "  make build-all     # Everything including llama.cpp"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test          # Run tests"
	@echo "  make test-cover   # Run tests with coverage"
	@echo "  make lint         # Run linter"
	@echo "  make fmt          # Format code"
	@echo "  make vet          # Run go vet"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  make clean        # Clean build artifacts"
	@echo "  make clean-all    # Clean everything"
	@echo ""
	@echo "▶️  Run:"
	@echo "  $(BIN_DIR)/ocg start      # Start all services"
	@echo "  $(BIN_DIR)/ocg stop       # Stop all services"
	@echo "  $(BIN_DIR)/ocg status     # Show status"
	@echo "  $(BIN_DIR)/ocg restart    # Restart services"
	@echo ""
	@echo "⚙️  Options:"
	@echo "  STATIC=1 make     # Static binary build"
	@echo "  LLAMA_JOBS=4 make build-llama  # Parallel llama.cpp build"
