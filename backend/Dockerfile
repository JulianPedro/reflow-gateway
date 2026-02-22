# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git npm

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates curl

# Copy binary from builder
COPY --from=builder /app/server /app/server

# Copy migrations (embedded in binary, but keeping for reference)
COPY --from=builder /app/internal/database/migrations /app/migrations

# Expose port
EXPOSE 3000

# Run the application
CMD ["/app/server", "-config", "/app/config.yaml"]
