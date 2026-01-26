# TikTok Shop Sync Server

Hệ thống đồng bộ đơn hàng và inventory giữa Local Database với TikTok Shop.

## 📋 Tính năng chính

- **Order Sync**: Đồng bộ đơn hàng real-time từ TikTok Shop qua Webhook
- **Inventory Sync**: Đẩy inventory từ local lên TikTok Shop
- **Fulfillment**: Ship đơn hàng và upload tracking number
- **OAuth Integration**: Kết nối nhiều shop TikTok

---

## 🚀 Quick Start

### 1. Cài đặt Dependencies

```powershell
# Clone repository
git clone https://github.com/NVMau/tiktok.git
cd sync-tiktok-mps

# Cài đặt Go modules
go mod download
```

### 2. Cấu hình Environment

```powershell
# Copy file config mẫu
cp .env.example .env

# Chỉnh sửa .env với thông tin của bạn
```

**Các biến bắt buộc:**

| Biến | Mô tả |
|------|-------|
| `TIKTOK_APP_KEY` | App Key từ TikTok Developer |
| `TIKTOK_APP_SECRET` | App Secret từ TikTok Developer |
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |

### 3. Khởi động Services

```powershell
# Start Docker services (Postgres, Redis, Ngrok)
docker-compose -f docker-compose.dev.yml up -d

# Build và chạy API
go build -o sync-api.exe ./cmd/api/
.\sync-api.exe

# (Terminal khác) Chạy Worker cho background jobs
go build -o sync-worker.exe ./cmd/worker/
.\sync-worker.exe
```

**Hoặc dùng script tự động:**

```powershell
.\start-dev.ps1
```

### 4. Kiểm tra Server

```powershell
curl http://localhost:8000/healthz
# Output: {"status":"ok","timestamp":"..."}
```

---

## 🔐 Kết nối TikTok Shop

### Bước 1: Lấy Authorization URL

```powershell
curl http://localhost:8000/api/v1/admin/auth/url
```

Response:
```json
{
  "url": "https://auth.tiktok-shops.com/oauth/authorize?app_key=xxx&..."
}
```

### Bước 2: Authorize trong Browser

1. Mở URL trên trong browser
2. Đăng nhập TikTok Shop
3. Chấp nhận quyền truy cập
4. Copy `code` từ URL callback

### Bước 3: Exchange Token

```powershell
curl "http://localhost:8000/api/v1/admin/auth/callback?code=YOUR_CODE_HERE"
```

### Bước 4: Kiểm tra Shop đã kết nối

```powershell
curl http://localhost:8000/api/v1/admin/shops
```

---

## 📦 API Endpoints

### Health Check

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/healthz` | Server health check |

### Authorization

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/admin/auth/url` | Lấy TikTok authorization URL |
| GET | `/api/v1/admin/auth/callback?code=xxx` | Exchange OAuth code |

### Shop Management

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/admin/shops` | Danh sách shops đã kết nối |
| GET | `/api/v1/admin/shops/:id` | Chi tiết shop |
| POST | `/api/v1/admin/shops/:id/refresh-token` | Refresh access token |
| POST | `/api/v1/admin/shops/:id/test` | Test kết nối API |
| GET | `/api/v1/admin/shops/:shop_id/products` | Lấy products từ TikTok |
| GET | `/api/v1/admin/shops/:shop_id/orders` | Lấy orders từ TikTok |
| POST | `/api/v1/admin/shops/:shop_id/orders/sync` | Sync orders vào DB |

### Orders

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/orders` | Danh sách orders trong DB |
| GET | `/api/v1/orders/:id` | Chi tiết order |
| GET | `/api/v1/orders/pending` | Orders chờ xử lý |
| GET | `/api/v1/orders/manual-review` | Orders cần review tay |
| GET | `/api/v1/orders/stats` | Thống kê orders |
| POST | `/api/v1/orders/:id/sync` | Sync order cụ thể |

### Webhooks

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| POST | `/api/v1/webhooks/tiktok` | Nhận webhook từ TikTok |
| GET | `/api/v1/webhooks/events` | Danh sách webhook events |
| POST | `/api/v1/webhooks/events/:id/retry` | Retry event thất bại |

### Inventory

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/inventory` | Danh sách inventory |
| GET | `/api/v1/inventory/:id` | Chi tiết inventory |
| GET | `/api/v1/inventory/stats` | Thống kê inventory |
| PUT | `/api/v1/shops/:shop_id/inventory/:sim_id` | Update local inventory |
| POST | `/api/v1/shops/:shop_id/inventory/:sim_id/sync` | Sync lên TikTok |
| POST | `/api/v1/shops/:shop_id/inventory/sync-all` | Sync tất cả dirty |

### Fulfillment

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/shops/:shop_id/orders/ready-to-ship` | Orders sẵn sàng ship |
| GET | `/api/v1/shops/:shop_id/orders/in-transit` | Orders đang vận chuyển |
| POST | `/api/v1/shops/:shop_id/orders/:order_id/ship` | Ship order (sync) |
| POST | `/api/v1/shops/:shop_id/orders/:order_id/ship-async` | Ship order (async) |
| GET | `/api/v1/shops/:shop_id/shipping-providers` | Danh sách carriers |
| GET | `/api/v1/shops/:shop_id/warehouses` | Danh sách warehouses |

### Product Mappings

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/v1/mappings` | Danh sách product mappings |
| POST | `/api/v1/shops/:shop_id/mappings` | Tạo mapping mới |

---

## 🧪 Testing

### Chạy tất cả Unit Tests

```powershell
go test ./... -v
```

### Chạy test cho từng package

```powershell
# Test services
go test ./internal/services/... -v

# Test handlers
go test ./internal/handlers/... -v

# Test TikTok client
go test ./internal/tiktok/... -v

# Test models
go test ./internal/models/... -v

# Test signature utils
go test ./pkg/signature/... -v
```

### Chạy test với coverage

```powershell
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Benchmark tests

```powershell
go test ./internal/handlers/... -bench=. -benchmem
```

---

## 🔄 Testing Full Flow

### Flow 1: Order Sync từ TikTok → Local

```powershell
# 1. Kiểm tra shop đã kết nối
curl http://localhost:8000/api/v1/admin/shops
# Lấy shop_id từ response

# 2. Lấy orders từ TikTok API
curl http://localhost:8000/api/v1/admin/shops/{shop_id}/orders

# 3. Sync orders vào database
curl -X POST http://localhost:8000/api/v1/admin/shops/{shop_id}/orders/sync

# 4. Kiểm tra orders đã sync
curl http://localhost:8000/api/v1/orders

# 5. Xem thống kê
curl http://localhost:8000/api/v1/orders/stats
```

### Flow 2: Webhook Processing

```powershell
# 1. Lấy ngrok URL (nếu dùng dev mode)
curl http://localhost:4040/api/tunnels
# Webhook URL: https://xxx.ngrok-free.app/api/v1/webhooks/tiktok

# 2. Cấu hình URL này trong TikTok Developer Console

# 3. Khi có order mới, webhook sẽ được gửi tự động

# 4. Kiểm tra webhook events đã nhận
curl http://localhost:8000/api/v1/webhooks/events

# 5. Nếu có event thất bại, retry
curl -X POST http://localhost:8000/api/v1/webhooks/events/{event_id}/retry
```

### Flow 3: Fulfillment (Ship đơn hàng)

```powershell
# 1. Lấy danh sách orders sẵn sàng ship
curl http://localhost:8000/api/v1/shops/{shop_id}/orders/ready-to-ship

# 2. Lấy danh sách shipping providers
curl http://localhost:8000/api/v1/shops/{shop_id}/shipping-providers

# 3. Ship order
curl -X POST http://localhost:8000/api/v1/shops/{shop_id}/orders/{order_id}/ship \
  -H "Content-Type: application/json" \
  -d '{"tracking_number":"VN123456789","shipping_provider_id":"provider_id"}'

# 4. Kiểm tra orders đang vận chuyển
curl http://localhost:8000/api/v1/shops/{shop_id}/orders/in-transit
```

### Flow 4: Inventory Sync (Local → TikTok)

```powershell
# 1. Tạo product mapping
curl -X POST http://localhost:8000/api/v1/shops/{shop_id}/mappings \
  -H "Content-Type: application/json" \
  -d '{"sim_id":"local_sim_123","tiktok_product_id":"tiktok_prod_456","tiktok_sku_id":"sku_789"}'

# 2. Update inventory local
curl -X PUT http://localhost:8000/api/v1/shops/{shop_id}/inventory/local_sim_123 \
  -H "Content-Type: application/json" \
  -d '{"quantity":10}'

# 3. Sync lên TikTok
curl -X POST http://localhost:8000/api/v1/shops/{shop_id}/inventory/local_sim_123/sync

# 4. Sync tất cả inventory dirty
curl -X POST http://localhost:8000/api/v1/shops/{shop_id}/inventory/sync-all
```

---

## 📊 Order Status Flow

| TikTok Status | Local Status | Mô tả |
|---------------|--------------|-------|
| UNPAID | PENDING_PAYMENT | Chờ thanh toán |
| ON_HOLD | PAID_HOLD | Đã thanh toán, chờ xử lý |
| AWAITING_SHIPMENT | READY_TO_SHIP | Sẵn sàng giao hàng |
| AWAITING_COLLECTION | SHIPPED_AWAITING_PICKUP | Đang chờ lấy hàng |
| IN_TRANSIT | IN_TRANSIT | Đang vận chuyển |
| DELIVERED | DELIVERED | Đã giao hàng |
| COMPLETED | COMPLETED | Hoàn thành |
| CANCELLED | CANCELLED | Đã hủy |

---

## 🗂️ Project Structure

```
sync-tiktok-mps/
├── cmd/
│   ├── api/main.go          # API server entry point
│   └── worker/main.go       # Asynq worker entry point
├── internal/
│   ├── config/              # App configuration
│   ├── database/            # DB connection + migrations
│   ├── models/              # GORM models
│   ├── tiktok/              # TikTok API client
│   │   ├── client.go        # Base HTTP client
│   │   ├── auth.go          # OAuth flow
│   │   ├── orders.go        # Order API
│   │   ├── products.go      # Product API
│   │   ├── fulfillment.go   # Fulfillment API
│   │   └── logistics.go     # Logistics API
│   ├── services/            # Business logic
│   │   ├── order_sync.go    # Order sync service
│   │   ├── inventory_sync.go# Inventory sync service
│   │   └── fulfillment_sync.go
│   ├── handlers/            # HTTP handlers
│   │   ├── admin.go         # Admin endpoints
│   │   ├── orders.go        # Order endpoints
│   │   ├── webhook.go       # Webhook receiver
│   │   └── ...
│   └── workers/             # Background job handlers
├── pkg/
│   └── signature/           # HMAC signing utilities
├── docker-compose.yml       # Production Docker config
├── docker-compose.dev.yml   # Development Docker config
└── .env                     # Environment variables
```

---

## 🐳 Docker Commands

```powershell
# Start development services
docker-compose -f docker-compose.dev.yml up -d

# View logs
docker-compose -f docker-compose.dev.yml logs -f

# Stop services
docker-compose -f docker-compose.dev.yml down

# Reset database (xóa data)
docker-compose -f docker-compose.dev.yml down -v
docker-compose -f docker-compose.dev.yml up -d
```

---

## ⚠️ Troubleshooting

### Lỗi: "connection refused" khi connect DB

```powershell
# Kiểm tra postgres đang chạy
docker ps | findstr postgres

# Kiểm tra logs
docker logs tiktok_sync_postgres
```

### Lỗi: Webhook không nhận được

1. Kiểm tra ngrok đang chạy: `http://localhost:4040`
2. Kiểm tra URL đã cấu hình đúng trong TikTok Developer Console
3. Kiểm tra webhook events: `curl http://localhost:8000/api/v1/webhooks/events`

### Lỗi: Token expired

```powershell
# Refresh token thủ công
curl -X POST http://localhost:8000/api/v1/admin/shops/{shop_id}/refresh-token
```

### Lỗi: Test failed

```powershell
# Chạy test với verbose output
go test ./... -v

# Chạy test cụ thể
go test -run TestOrderSyncService ./internal/services/... -v
```

---

## 📝 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8000 | Server port |
| `ENVIRONMENT` | development | Environment mode |
| `LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `REDIS_URL` | localhost:6379 | Redis connection string |
| `TIKTOK_APP_KEY` | - | TikTok App Key |
| `TIKTOK_APP_SECRET` | - | TikTok App Secret |
| `TIKTOK_WEBHOOK_SECRET` | - | Webhook signature secret |

---

## 📜 License

MIT License
