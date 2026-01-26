# TikTok Shop Sync Server

A Go-based synchronization server for TikTok Shop integration, handling real-time order sync, inventory management, and fulfillment operations.

## Features

- **Order Sync** – Real-time order synchronization via TikTok webhooks
- **Inventory Management** – Push local inventory to TikTok Shop
- **Fulfillment** – Ship orders and upload tracking numbers
- **Multi-Shop** – OAuth integration supporting multiple TikTok shops

## Tech Stack

- **Go + Fiber** – HTTP server
- **PostgreSQL** – Primary database
- **Redis + Asynq** – Background job processing
- **GORM** – ORM

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- TikTok Shop Developer account

### Setup

```bash
# Clone and install dependencies
git clone https://github.com/NVMau/tiktok.git
cd tiktok
go mod download

# Configure environment
cp .env.example .env
# Edit .env with your credentials

# Start services
docker-compose -f docker-compose.dev.yml up -d

# Run API server
go run ./cmd/api

# Run worker (separate terminal)
go run ./cmd/worker
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `TIKTOK_APP_KEY` | TikTok Developer App Key |
| `TIKTOK_APP_SECRET` | TikTok Developer App Secret |
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |

## Project Structure

```
├── cmd/
│   ├── api/main.go           # API server
│   └── worker/main.go        # Background worker
├── internal/
│   ├── config/               # Configuration
│   ├── database/             # Database connection
│   ├── models/               # GORM models
│   ├── tiktok/               # TikTok API client
│   ├── services/             # Business logic
│   ├── handlers/             # HTTP handlers
│   └── workers/              # Asynq task handlers
├── pkg/
│   └── signature/            # HMAC signing utilities
└── migrations/               # SQL migrations
```

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Order Status Mapping

| TikTok Status | Local Status |
|---------------|--------------|
| UNPAID | PENDING_PAYMENT |
| ON_HOLD | PAID_HOLD |
| AWAITING_SHIPMENT | READY_TO_SHIP |
| IN_TRANSIT | IN_TRANSIT |
| DELIVERED | DELIVERED |
| COMPLETED | COMPLETED |
| CANCELLED | CANCELLED |

## License

MIT
