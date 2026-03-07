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
    -DLLAMA_BUILD_EMBEDDING=OFF \
    -DBUILD_SHARED_LIBS=OFF && \
    cmake --build . --config Release --target llama-server -j $(nproc)

# ==========================================
# Stage 2: Final Runtime Image
# ==========================================
FROM alpine:3.19

# Install runtime dependencies (libstdc++ and libgomp are required by statically built llama-server)
RUN apk add --no-cache ca-certificates tzdata sqlite libstdc++ libgomp

WORKDIR /app

# Copy compiled llama-server binary from the builder stage
COPY --from=cpp-builder /src/llama.cpp/build/bin/llama-server /app/

# Create required directories
RUN mkdir -p /app/models

# Set environment variables
ENV LLAMA_SERVER_BIN=/app/llama-server

# Persist models directory
VOLUME ["/app/models"]

EXPOSE 8080

# Run llama-server by default
ENTRYPOINT ["/app/llama-server"]
CMD ["--help"]
