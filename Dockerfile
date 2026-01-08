# Build stage
FROM golang:1.24-alpine3.23 AS builder

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates=20251003-r0

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gomodel ./cmd/gomodel

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary and ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /gomodel /gomodel
COPY --from=builder /app/config/*.yaml /app/config

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["/gomodel"]
