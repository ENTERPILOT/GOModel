# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gomodel ./cmd/gomodel

# Runtime stage
FROM gcr.io/distroless/static-debian12

# Copy binary and ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /gomodel /gomodel
COPY --from=builder /app/config /app/config

# Create cache directory (writable by non-root user)
WORKDIR /app

# Run as non-root user
USER 1000

EXPOSE 8080

ENTRYPOINT ["/gomodel"]
