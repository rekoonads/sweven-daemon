# Daemon Dockerfile
FROM golang:1.18-alpine AS builder

WORKDIR /app

# Copy source
COPY . .

# Build
RUN go mod download 2>/dev/null || true
RUN go build -o daemon .

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/daemon .

CMD ["./daemon"]
