FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build API binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

# Build Worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/worker

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binaries
COPY --from=builder /api /app/api
COPY --from=builder /worker /app/worker

# Default command (can be overridden)
CMD ["/app/api"]
