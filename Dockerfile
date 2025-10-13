# Use build arguments for multi-architecture support
ARG TARGETPLATFORM
ARG BUILDPLATFORM

# Build arguments for versioning and metadata
ARG VERSION="unknown"
ARG BUILDTIME="unknown"
ARG COMMIT_SHA="unknown"

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

# Install necessary build dependencies
RUN apk add --no-cache git ca-certificates

# Set up build directory
WORKDIR /build

# Copy go modules first for better caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Extract target architecture and OS from TARGETPLATFORM
# TARGETPLATFORM format: linux/amd64, linux/arm64, etc.
ARG TARGETPLATFORM
ARG VERSION
ARG BUILDTIME
ARG COMMIT_SHA

RUN mkdir -p bin && \
    case "$TARGETPLATFORM" in \
        "linux/amd64") export GOARCH=amd64 ;; \
        "linux/arm64") export GOARCH=arm64 ;; \
        "linux/arm/v7") export GOARCH=arm && export GOARM=7 ;; \
        "linux/arm/v6") export GOARCH=arm && export GOARM=6 ;; \
        *) echo "Unsupported platform: $TARGETPLATFORM" && exit 1 ;; \
    esac && \
    echo "Building for GOOS=linux GOARCH=$GOARCH ${GOARM:+GOARM=$GOARM}" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH ${GOARM:+GOARM=$GOARM} \
    go build -a -installsuffix cgo \
    -o bin/webserver cmd/web/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH ${GOARM:+GOARM=$GOARM} \
    go build -a -installsuffix cgo \
    -o bin/server cmd/server/main.go

# Final stage - use the target platform
FROM alpine:latest

# Metadata labels for better image identification
ARG VERSION
ARG BUILDTIME  
ARG COMMIT_SHA
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.created="${BUILDTIME}"
LABEL org.opencontainers.image.revision="${COMMIT_SHA}"
LABEL org.opencontainers.image.title="AI Infrastructure Agent"
LABEL org.opencontainers.image.description="AI-powered infrastructure discovery and management agent"
LABEL org.opencontainers.image.vendor="VersusControl"
LABEL org.opencontainers.image.source="https://github.com/VersusControl/ai-infrastructure-agent"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

# Copy configuration files and binaries
COPY --from=build /build/web web/
COPY --from=build /build/settings settings/
COPY --from=build /build/config.yaml ./
COPY --from=build /build/bin/webserver ./
COPY --from=build /build/bin/server ./

# Set proper permissions
RUN chmod +x /app/webserver /app/server && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Use the webserver binary as the default command
CMD ["/app/webserver"]