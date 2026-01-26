# TikTok Shop Sync Server - Implementation Plan

## 📋 Tổng quan dự án

**Mục tiêu:**
1. Cập nhật trạng thái đơn hàng real-time (Order Management)
2. Thêm/bớt sản phẩm từ kho hàng local lên TikTok Shop (Inventory Sync)

**Tech Stack:**
- Server: **Go + Fiber**
- Database: PostgreSQL (Docker)
- Queue: Redis + Asynq (Go task queue)
- ORM: GORM

---

## 🏗️ Architecture

```
┌─────────────────┐     ┌──────────────────────────────────────┐
│  TikTok Shop    │     │         Sync Server (Go Fiber)       │
│  ─────────────  │     │  ────────────────────────────────    │
│  - Webhooks ────┼────►│  POST /webhooks/tiktok               │
│  - Order API    │     │       │                              │
│  - Product API  │◄────┼───────┼── TikTok API Client          │
│  - Fulfillment  │     │       ▼                              │
└─────────────────┘     │  ┌─────────┐    ┌─────────────────┐  │
                        │  │ Redis   │───►│ Asynq Worker    │  │
                        │  └─────────┘    └────────┬────────┘  │
                        │                          │           │
                        │  ┌───────────────────────▼────────┐  │
                        │  │      PostgreSQL                │  │
                        │  │  ┌──────────────────────────┐  │  │
                        │  │  │ tiktok_sync schema       │  │  │
                        │  │  │ (orders, products, jobs) │  │  │
                        │  │  └──────────────────────────┘  │  │
                        │  │  ┌──────────────────────────┐  │  │
                        │  │  │ public schema (existing) │  │  │
                        │  │  │ (sim_numbers, sim_orders)│  │  │
                        │  │  └──────────────────────────┘  │  │
                        │  └────────────────────────────────┘  │
                        └──────────────────────────────────────┘
```

---

## 📊 Database Schema (tiktok_sync)

### Bảng cấu hình
| Bảng | Mô tả |
|------|-------|
| `shops` | Thông tin TikTok Shop (shop_id, region, status) |
| `oauth_tokens` | Access/refresh tokens cho mỗi shop |

### Bảng mapping
| Bảng | Mô tả |
|------|-------|
| `product_mappings` | Map local SIM ↔ TikTok product/SKU |
| `inventory_states` | Trạng thái tồn kho cần đồng bộ |

### Bảng orders (mirror)
| Bảng | Mô tả |
|------|-------|
| `orders` | Mirror orders từ TikTok |
| `order_items` | Chi tiết items trong order |

### Bảng job queue
| Bảng | Mô tả |
|------|-------|
| `webhook_events` | Inbox webhook events (idempotent) |
| `sync_jobs` | Outbox jobs để push lên TikTok |

---

## 🔄 Order Status Flow

```
TikTok Status          →  Local Status
─────────────────────────────────────────
UNPAID                 →  PENDING_PAYMENT
ON_HOLD (paid)         →  PAID_HOLD
AWAITING_SHIPMENT      →  READY_TO_SHIP
AWAITING_COLLECTION    →  SHIPPED_AWAITING_PICKUP
IN_TRANSIT             →  IN_TRANSIT
DELIVERED              →  DELIVERED
COMPLETED              →  COMPLETED
CANCELLED              →  CANCELLED
```

---

## 📁 Project Structure

```
sync-tiktok-mps/
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── cmd/
│   ├── api/
│   │   └── main.go             # API server entry
│   └── worker/
│       └── main.go             # Asynq worker entry
├── internal/
│   ├── config/
│   │   └── config.go           # App configuration
│   ├── database/
│   │   ├── database.go         # DB connection
│   │   └── migrations.go       # Auto migrations
│   ├── models/
│   │   ├── shop.go
│   │   ├── oauth.go
│   │   ├── product.go
│   │   ├── order.go
│   │   └── job.go
│   ├── tiktok/
│   │   ├── client.go           # API client base
│   │   ├── auth.go             # OAuth flow
│   │   ├── products.go         # Product API
│   │   ├── orders.go           # Order API
│   │   ├── fulfillment.go      # Fulfillment API
│   │   └── logistics.go        # Logistics API
│   ├── services/
│   │   ├── order_sync.go       # Order sync logic
│   │   ├── inventory_sync.go   # Inventory sync logic
│   │   └── mapping.go          # SKU mapping
│   ├── handlers/
│   │   ├── webhook.go          # Webhook handlers
│   │   ├── health.go           # Health check
│   │   └── admin.go            # Admin endpoints
│   └── workers/
│       ├── asynq.go            # Asynq setup
│       └── tasks.go            # Task handlers
├── pkg/
│   └── utils/
│       └── signature.go        # HMAC signing utils
└── tests/
    └── ...
```

---

## 🚀 Implementation Phases

### Phase 0: Bootstrap (1h) ✅ DONE
- [x] Create PLAN.md
- [x] Docker compose (Postgres, Redis)
- [x] Dockerfile for Go
- [x] go.mod + dependencies
- [x] Project structure
- [x] Database models + auto migration
- [x] Basic Fiber app + health check
- [x] Webhook endpoint (basic)
- [x] Asynq worker setup

### Phase 1: TikTok Auth + Client (1-3h) ✅ DONE
- [x] Config settings (app_key, app_secret)
- [x] Request signing (HMAC-SHA256) - `pkg/signature/signature.go`
- [x] Token management (refresh flow) - `internal/tiktok/auth.go`
- [x] Base API client with retry/rate-limit - `internal/tiktok/client.go`
- [x] Shop & OAuth models
- [x] Orders API - `internal/tiktok/orders.go`
- [x] Products API - `internal/tiktok/products.go`
- [x] Fulfillment API - `internal/tiktok/fulfillment.go`
- [x] Logistics API - `internal/tiktok/logistics.go`
- [x] Shops API - `internal/tiktok/shops.go`
- [x] Admin handlers (auth callback, shop management)

### Phase 2: Webhook Ingestion (1-3h) ✅ DONE
- [x] POST /webhooks/tiktok endpoint
- [x] Signature verification (Tiktok-Signature header)
- [x] webhook_events table + idempotency (event_id unique)
- [x] Asynq task queue setup (with retry, priority queues)
- [x] Worker task: process ORDER_STATUS_CHANGE, ORDER_CREATED
- [x] Order upsert logic (fetch full detail from API)
- [x] Scheduled reconciliation (every 6 hours)
- [x] Admin endpoints: list events, retry failed events

### Phase 3: Order Sync (1-2 days) ✅ DONE
- [x] Order API client (Get Order List, Get Order Details)
- [x] Order mirror tables (orders, order_items)
- [x] Webhook → Order sync logic
- [x] Map TikTok order → Local SIM via product_mapping
- [x] Reserve SIM inventory (transaction lock)
- [x] Status mapping (TikTok → Local)
- [x] Order handlers (list, detail, sync, stats)
- [x] Manual review workflow

### Phase 4: Inventory/Product Push (1-2 days) ✅ DONE
- [x] Product API client (Create, Update, Inventory)
- [x] product_mappings table
- [x] inventory_states table
- [x] Local change detection (dirty flag)
- [x] Batch push jobs (sync-all endpoint)
- [x] Inventory handlers (CRUD, sync)
- [x] Product mapping CRUD

### Phase 5: Fulfillment Push (1-3h) ✅ DONE
- [x] Fulfillment API client (Ship Package)
- [x] Logistics API (Get carriers/warehouses)
- [x] Ship order endpoint (sync & async)
- [x] Tracking number upload
- [x] Ready-to-ship / In-transit order lists

### Phase 6: Reconciliation + Hardening (1-3h) ✅ DONE
- [x] Scheduled order reconciliation (every 6 hours)
- [x] Dead-letter job handling (retry endpoint)
- [x] Admin requeue endpoint
- [x] Comprehensive logging
- [x] Stats endpoints (orders, inventory)

---

## ⚠️ Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Bán trùng SIM | DB transaction + row lock khi reserve |
| Webhook duplicate | Idempotency key + dedupe |
| Rate limit API | Exponential backoff + dead-letter |
| Mapping SKU sai | Chuẩn hóa seller_sku = `SIM-{sim_id}` |
| Webhook miss | Scheduled reconciliation job |

---

## 🔐 TikTok API Scopes Cần Thiết

| Scope | Mô tả |
|-------|-------|
| Order Information | Access order data real-time |
| Fulfillment Basic | Manage orders, update status |
| Product Basic | Sync product info |
| Product Modify | Edit products, sync inventory |
| Logistics Basic | Get warehouse/carrier info |

---

## 📝 Notes

- Mỗi SIM số đẹp = 1 SKU, qty = 0 hoặc 1
- Dùng schema `tiktok_sync` riêng biệt để tránh conflict
- Webhook chỉ trigger, luôn fetch Order Details để lấy data đầy đủ
- Tất cả jobs phải idempotent và retry-safe

---

## 🛠️ Commands

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f api
docker-compose logs -f worker

# Run migrations (auto via GORM)
# Migrations run automatically on startup

# Stop services
docker-compose down
```

---

## 🔗 API Endpoints

### Health Check
- `GET /healthz` - Server health check

### Webhooks
- `POST /api/v1/webhooks/tiktok` - TikTok webhook receiver

### Admin - Authorization
- `GET /api/v1/admin/auth/url` - Get TikTok authorization URL
- `GET /api/v1/admin/auth/callback?code=xxx` - OAuth callback

### Admin - Shop Management
- `GET /api/v1/admin/shops` - List all connected shops
- `GET /api/v1/admin/shops/:id` - Get shop details
- `POST /api/v1/admin/shops/:id/refresh-token` - Refresh shop token
- `POST /api/v1/admin/shops/:id/test` - Test shop API connection

### Webhook Events
- `GET /api/v1/webhooks/events?status=pending&limit=50` - List webhook events
- `POST /api/v1/webhooks/events/:id/retry` - Retry failed event

### Orders
- `GET /api/v1/orders` - List orders
- `GET /api/v1/orders/:id` - Get order detail
- `GET /api/v1/orders/pending` - Get pending sync orders
- `GET /api/v1/orders/manual-review` - Get manual review orders
- `GET /api/v1/orders/stats` - Get order statistics
- `POST /api/v1/orders/:id/sync` - Sync order to local

### Inventory
- `GET /api/v1/inventory` - List inventory states
- `GET /api/v1/inventory/:id` - Get inventory detail
- `GET /api/v1/inventory/stats` - Get inventory statistics
- `PUT /api/v1/shops/:shop_id/inventory/:sim_id` - Update local inventory
- `POST /api/v1/shops/:shop_id/inventory/:sim_id/sync` - Sync to TikTok
- `POST /api/v1/shops/:shop_id/inventory/sync-all` - Sync all dirty inventory

### Product Mappings
- `GET /api/v1/mappings` - List product mappings
- `POST /api/v1/shops/:shop_id/mappings` - Create mapping

### Fulfillment
- `GET /api/v1/shops/:shop_id/orders/ready-to-ship` - Orders ready to ship
- `GET /api/v1/shops/:shop_id/orders/in-transit` - Orders in transit
- `POST /api/v1/shops/:shop_id/orders/:order_id/ship` - Ship order (sync)
- `POST /api/v1/shops/:shop_id/orders/:order_id/ship-async` - Ship order (async)
- `GET /api/v1/shops/:shop_id/shipping-providers` - Get carriers
- `GET /api/v1/shops/:shop_id/warehouses` - Get warehouses

---

## 🔑 Authorization Flow

1. Gọi `GET /api/v1/admin/auth/url` để lấy authorization URL
2. Mở URL đó trong browser, đăng nhập TikTok Shop và authorize
3. TikTok redirect về callback URL với `?code=xxx`
4. Gọi `GET /api/v1/admin/auth/callback?code=xxx` để exchange token
5. Server tự động lưu shop info + tokens vào database

---

## ✅ Testing Status (Updated: 2026-01-23)

### APIs Tested & Working

| Endpoint | Status | Notes |
|----------|--------|-------|
| `GET /healthz` | ✅ OK | Health check |
| `GET /api/v1/admin/auth/url` | ✅ OK | Returns TikTok auth URL |
| `GET /api/v1/admin/auth/callback` | ✅ OK | OAuth exchange working |
| `GET /api/v1/admin/shops` | ✅ OK | Lists connected shops |
| `GET /api/v1/admin/shops/:id` | ✅ OK | Shop details |
| `GET /api/v1/admin/shops/:shop_id/products` | ✅ OK | Fetches products from TikTok |
| `GET /api/v1/admin/shops/:shop_id/orders` | ✅ OK | Fetches orders from TikTok |
| `POST /api/v1/admin/shops/:shop_id/orders/sync` | ✅ OK | Syncs orders to DB |
| `GET /api/v1/orders` | ✅ OK | Lists orders from DB |
| `GET /api/v1/orders/stats` | ✅ OK | Order statistics |
| `POST /api/v1/webhooks/tiktok` | ✅ OK | Receives TikTok webhooks |
| `GET /api/v1/webhooks/events` | ✅ OK | Lists webhook events |

### TikTok Shop Integration

| Feature | Status | Notes |
|---------|--------|-------|
| OAuth Authorization | ✅ OK | Shop "Shop Test For MPS" (VN) connected |
| Get Products | ✅ OK | Returns 3 products |
| Get Orders | ✅ OK | Fetches from TikTok API |
| Sync Orders to DB | ✅ OK | Full order detail saved |
| Webhook Receive | ✅ OK | ORDER_STATUS_CHANGE working |
| Webhook Auto-Process | 🔄 Pending | Worker ready, needs testing |

### Shop Connected

| Shop ID | Name | Region | Status |
|---------|------|--------|--------|
| 7494377304019011256 | Shop Test For MPS | VN | Active ✅ |

### Known Issues Fixed

| Issue | Fix Applied |
|-------|-------------|
| Products API: "PageSize is a required field" | `page_size` moved to query params |
| Orders API: "Invalid method" | Changed to POST method |
| Orders API: "SortField is invalid" | Changed to lowercase `create_time` |
| Orders Detail: "Invalid path" | Changed to `/order/202309/orders?ids=xxx` |
| Webhook: 400 Bad Request | Fixed `type` from string to int |

### Not Yet Tested

| Feature | Reason |
|---------|--------|
| Ship Order | No order ready for shipping |
| Inventory Sync | No product mappings created |
| Scheduled Reconciliation | Runs every 6 hours |
| Worker Auto-Processing | Worker needs to be started |

---

## 🚀 How to Run

### Development Mode

```powershell
# Terminal 1: Start dependencies (Postgres, Redis, Ngrok)
docker-compose -f docker-compose.dev.yml up -d

# Terminal 2: Start API server
.\sync-api.exe

# Terminal 3: Start Worker (for auto webhook processing)
.\sync-worker.exe
```

### Ngrok for Webhooks

```powershell
# Check ngrok URL
curl http://localhost:4040/api/tunnels

# Webhook URL format:
# https://xxx.ngrok-free.app/api/v1/webhooks/tiktok
```

### Quick Test Commands

```powershell
# Health check
curl http://localhost:8000/healthz

# List shops
curl http://localhost:8000/api/v1/admin/shops

# Get products
curl http://localhost:8000/api/v1/admin/shops/7494377304019011256/products

# Get orders from TikTok
curl http://localhost:8000/api/v1/admin/shops/7494377304019011256/orders

# Sync orders to DB
curl -X POST http://localhost:8000/api/v1/admin/shops/7494377304019011256/orders/sync

# List orders in DB
curl http://localhost:8000/api/v1/orders

# List webhook events
curl http://localhost:8000/api/v1/webhooks/events
```
