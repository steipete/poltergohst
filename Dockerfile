# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o poltergeist cmd/poltergeist/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates git

# Create non-root user
RUN addgroup -g 1000 poltergeist && \
    adduser -D -u 1000 -G poltergeist poltergeist

# Set working directory
WORKDIR /workspace

# Copy binary from builder
COPY --from=builder /build/poltergeist /usr/local/bin/poltergeist

# Set ownership
RUN chown -R poltergeist:poltergeist /workspace

# Switch to non-root user
USER poltergeist

# Set entrypoint
ENTRYPOINT ["poltergeist"]

# Default command
CMD ["--help"]