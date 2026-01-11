# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o wikigraph ./cmd/wikigraph

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/wikigraph /app/wikigraph

# Create non-root user
RUN addgroup -g 1000 wikigraph && \
    adduser -D -u 1000 -G wikigraph wikigraph

# Set ownership
RUN chown -R wikigraph:wikigraph /app

USER wikigraph

EXPOSE 8080

ENTRYPOINT ["/app/wikigraph"]
CMD ["serve"]
