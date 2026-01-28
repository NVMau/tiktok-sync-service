# TikTok Sync Server - API Documentation

## Base URL
```
http://localhost:8000
```

---

## 🏥 Health Check

### GET /healthz
Kiểm tra trạng thái server.

**Response:**
```json
{
  "status": "ok",
  "service": "tiktok-sync-server"
}
```

---

## 🔐 Authorization APIs

### GET /api/v1/admin/auth/url
Lấy URL để authorize TikTok Shop.

**Query Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| state | string | No | State parameter để verify callback |

**Response:**
```json
{
  "authorization_url": "https://services.tiktokshop.com/open/authorize?service_id=xxx&state=xxx",
  "instructions": "Visit this URL to authorize your TikTok Shop..."
}
```

**Flow:**
1. Gọi API này để lấy URL
2. Mở URL trong browser
3. Đăng nhập TikTok Shop và chấp nhận authorize
4. TikTok redirect về callback URL với `?code=xxx`

---

### GET /api/v1/admin/auth/callback
Xử lý OAuth callback từ TikTok, exchange auth_code lấy access_token.

**Query Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| code | string | Yes | Authorization code từ TikTok |
| state | string | No | State để verify |

**Response Success:**
```json
{
  "message": "authorization successful",
  "shops": [
    {
      "id": 1,
      "shop_id": "7369437808455026474",
      "shop_name": "My Shop",
      "region": "VN",
      "status": "active"
    }
  ],
  "seller": "Seller Name",
  "region": "VN",
  "scopes": ["order.read", "order.write", "product.read", "product.write"]
}
```

**Response Error:**
```json
{
  "error": "authorization denied or auth_code missing"
}
```

---

## 🏪 Shop Management APIs

### GET /api/v1/admin/shops
Liệt kê tất cả shops đã kết nối.

**Response:**
```json
{
  "shops": [
    {
      "id": 1,
      "shop_id": "7369437808455026474",
      "shop_name": "My Shop",
      "region": "VN",
      "status": "active",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

### GET /api/v1/admin/shops/:id
Lấy chi tiết một shop.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | TikTok Shop ID |

**Response:**
```json
{
  "shop": {
    "id": 1,
    "shop_id": "7369437808455026474",
    "shop_name": "My Shop",
    "region": "VN",
    "status": "active"
  },
  "token": {
    "expires_at": "2024-01-08T00:00:00Z",
    "scopes": "[\"order.read\",\"product.write\"]",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

---

### POST /api/v1/admin/shops/:id/refresh-token
Làm mới access token của shop.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | TikTok Shop ID |

**Response:**
```json
{
  "message": "token refreshed successfully",
  "access_token": "TTP_xxxxxxxxxxxxx..."
}
```

---

### POST /api/v1/admin/shops/:id/test
Test kết nối API với TikTok Shop.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | TikTok Shop ID |

**Request Body:**
```json
{
  "shop_cipher": "ROW_xxxxxx"
}
```

> **Note:** `shop_cipher` lấy từ API Get Authorized Shops của TikTok

**Response Success:**
```json
{
  "message": "connection successful",
  "warehouses": [
    {
      "id": "7012345678901234567",
      "name": "Default Warehouse",
      "type": "SALES_WAREHOUSE",
      "is_default": true
    }
  ]
}
```

---

### GET /api/v1/admin/shops/:shop_id/products
Lấy danh sách sản phẩm của shop từ TikTok.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | TikTok Shop ID |
  
---

### GET /api/v1/admin/shops/:shop_id/orders
Lấy danh sách đơn hàng của shop từ TikTok.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | TikTok Shop ID |

---

### POST /api/v1/admin/shops/:shop_id/orders/sync
Đồng bộ đơn hàng của shop từ TikTok về database.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | TikTok Shop ID |

---

## 📦 Orders APIs

### GET /api/v1/orders
Lấy danh sách tất cả đơn hàng trong database.

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| limit | int | 50 | Số đơn hàng tối đa |
| offset | int | 0 | Offset phân trang |
| status | string | - | Filter theo trạng thái |
| shop_id | string | - | Filter theo shop |

---

### GET /api/v1/orders/pending
Lấy danh sách đơn hàng đang pending (chưa xử lý).

---

### GET /api/v1/orders/manual-review
Lấy danh sách đơn hàng cần review thủ công.

---

### GET /api/v1/orders/stats
Lấy thống kê đơn hàng.

**Response:**
```json
{
  "total": 100,
  "pending": 10,
  "awaiting_shipment": 15,
  "in_transit": 30,
  "completed": 40,
  "cancelled": 5
}
```

---

### GET /api/v1/orders/:id
Lấy chi tiết một đơn hàng.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | Order ID |

---

### POST /api/v1/orders/:id/sync
Đồng bộ lại một đơn hàng từ TikTok.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | Order ID |

---

## 📦 Products & SKUs APIs

### GET /api/v1/shops/:shop_id/products
Lấy danh sách sản phẩm của shop trong database.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | TikTok Shop ID |

---

### POST /api/v1/shops/:shop_id/products/sync
Đồng bộ sản phẩm từ TikTok về database.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | TikTok Shop ID |

---

### GET /api/v1/products/:id
Lấy chi tiết một sản phẩm.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | Product ID |

---

### GET /api/v1/products/:product_id/skus
Lấy danh sách SKUs của sản phẩm.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| product_id | string | Product ID |

---

### POST /api/v1/products/:product_id/skus
Tạo một SKU mới cho sản phẩm.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| product_id | string | Product ID |

---

### POST /api/v1/products/:product_id/skus/batch
Tạo nhiều SKUs cho sản phẩm.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| product_id | string | Product ID |

---

### GET /api/v1/skus/:id
Lấy chi tiết một SKU.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | SKU ID |

---

### PUT /api/v1/skus/:id/price
Cập nhật giá của SKU.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | string | SKU ID |

---

## 📊 Inventory APIs

### POST /api/v1/shops/:shop_id/skus/:sku_id/sync
Đồng bộ inventory của SKU với TikTok.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |
| sku_id | string | SKU ID |

---

### POST /api/v1/shops/:shop_id/products/:product_id/sync-inventory
Đồng bộ inventory của tất cả SKUs trong sản phẩm.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |
| product_id | string | Product ID |

---

### GET /api/v1/shops/:shop_id/inventory/stats
Lấy thống kê inventory của shop.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |

---

### GET /api/v1/shops/:shop_id/skus/pending
Lấy danh sách SKUs pending (chưa đồng bộ).

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |

---

### POST /api/v1/shops/:shop_id/products/:product_id/push-skus
Push các SKUs pending lên TikTok.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |
| product_id | string | Product ID |

---

### POST /api/v1/shops/:shop_id/skus/:sku_id/push
Push một SKU lên TikTok.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |
| sku_id | string | SKU ID |

---

### GET /api/v1/inventory
Legacy: Lấy danh sách inventory.

---

### GET /api/v1/inventory/:id
Legacy: Lấy chi tiết inventory.

---

## 🚚 Fulfillment APIs

### GET /api/v1/shops/:shop_id/orders/ready-to-ship
Lấy danh sách đơn hàng sẵn sàng ship.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |

---

### GET /api/v1/shops/:shop_id/orders/in-transit
Lấy danh sách đơn hàng đang vận chuyển.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |

---

### POST /api/v1/shops/:shop_id/orders/:order_id/ship
Ship đơn hàng (đồng bộ).

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |
| order_id | string | Order ID |

**Request Body:**
```json
{
  "tracking_number": "VN123456789",
  "shipping_provider_id": "provider_001"
}
```

---

### POST /api/v1/shops/:shop_id/orders/:order_id/ship-async
Ship đơn hàng (bất đồng bộ - qua queue).

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |
| order_id | string | Order ID |

---

### GET /api/v1/shops/:shop_id/shipping-providers
Lấy danh sách nhà vận chuyển.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |

---

### GET /api/v1/shops/:shop_id/warehouses
Lấy danh sách kho hàng.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| shop_id | string | Shop ID |

---

## 📨 Webhook APIs

### POST /api/v1/webhooks/tiktok
Nhận webhook events từ TikTok Shop.

**Headers:**
| Header | Description |
|--------|-------------|
| Tiktok-Signature | `t=timestamp,s=signature` - Signature để verify |

**Request Body (Example - Order Status Update):**
```json
{
  "type": "ORDER_STATUS_CHANGE",
  "shop_id": "7369437808455026474",
  "timestamp": 1704067200,
  "data": {
    "order_id": "1234567890",
    "order_status": "AWAITING_SHIPMENT",
    "update_time": 1704067200,
    "shop_cipher": "ROW_xxxxxx"
  }
}
```

**Response:**
```json
{
  "message": "event received",
  "event_id": "abc123..."
}
```

**Webhook Event Types:**
| Type | Description | Processing |
|------|-------------|------------|
| ORDER_STATUS_CHANGE | Trạng thái đơn hàng thay đổi | Fetch order detail → Upsert DB |
| ORDER_CREATED | Đơn hàng mới được tạo | Fetch order detail → Upsert DB |
| PACKAGE_UPDATE | Package được cập nhật | Log (TODO) |
| PRODUCT_STATUS_CHANGE | Sản phẩm thay đổi trạng thái | Log (TODO) |
| RETURN_STATUS_CHANGE | Yêu cầu hoàn trả thay đổi | Log (TODO) |

**Processing Flow:**
1. Verify signature (nếu có webhook secret)
2. Validate timestamp (không quá 5 phút)
3. Tìm shop trong DB
4. Generate event_id (SHA256 hash) để idempotency
5. Lưu vào `webhook_events` table
6. Enqueue task vào Asynq (priority queue cho ORDER_STATUS_CHANGE)
7. Worker xử lý: fetch order detail từ TikTok API → upsert vào DB
8. **Callback** (nếu có CALLBACK_URL): Enqueue callback task → gửi thông tin đến external service

**Callback Payloads:**

Khi có CALLBACK_URL được cấu hình, hệ thống sẽ gửi POST request đến URL đó với các payload sau:

**ORDER_CREATED - Đơn hàng mới:**
```json
{
  "event_type": "ORDER_CREATED",
  "tiktok_order_id": "576xxxxxxx",
  "tiktok_shop_id": "7123456789",
  "tiktok_order_status": "AWAITING_PAYMENT",
  "total_amount": 299000,
  "currency": "VND",
  "placed_at": "2024-01-28T10:30:00Z",
  "items": [
    {
      "tiktok_order_item_id": "item_001",
      "tiktok_sku_id": "sku_123",
      "seller_sku": "0912345678",
      "qty": 1,
      "price": 149000
    },
    {
      "tiktok_order_item_id": "item_002",
      "tiktok_sku_id": "sku_456",
      "seller_sku": "0987654321",
      "qty": 1,
      "price": 150000
    }
  ],
  "timestamp": 1706430600
}
```

**ORDER_STATUS_CHANGE - Thay đổi trạng thái:**
```json
{
  "event_type": "ORDER_STATUS_CHANGE",
  "tiktok_order_id": "576xxxxxxx",
  "tiktok_shop_id": "7123456789",
  "old_status": "AWAITING_PAYMENT",
  "new_status": "AWAITING_SHIPMENT",
  "timestamp": 1706430900
}
```

**Callback Headers:**
| Header | Description |
|--------|-------------|
| Content-Type | application/json |
| X-Callback-Source | tiktok-sync-server |
| X-Callback-Timestamp | Unix timestamp |
| X-Callback-Signature | HMAC-SHA256 signature (nếu có CALLBACK_SECRET) |

**Callback Configuration (ENV):**
| Variable | Default | Description |
|----------|---------|-------------|
| CALLBACK_URL | (empty) | URL nhận callback, để trống để disable |
| CALLBACK_SECRET | (empty) | Secret key cho HMAC signature |
| CALLBACK_TIMEOUT | 10 | Timeout (seconds) |
| CALLBACK_RETRIES | 3 | Số lần retry nếu fail |

**Signature Verification (Bảo mật):**

Để verify callback là từ TikTok Sync Server, external service cần:

1. Lấy `X-Callback-Timestamp` và `X-Callback-Signature` từ headers
2. Validate timestamp không quá cũ (recommend: 5 phút)
3. Tính signature: `HMAC-SHA256(timestamp + "." + body, CALLBACK_SECRET)`
4. So sánh với `X-Callback-Signature`

**Ví dụ verify bằng Python:**
```python
import hmac
import hashlib
import time

def verify_callback(timestamp, body, signature, secret):
    # Check timestamp (5 minutes tolerance)
    if abs(time.time() - int(timestamp)) > 300:
        return False
    
    # Calculate expected signature
    message = f"{timestamp}.{body}"
    expected = hmac.new(
        secret.encode(),
        message.encode(),
        hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(expected, signature)
```

**Ví dụ verify bằng Node.js:**
```javascript
const crypto = require('crypto');

function verifyCallback(timestamp, body, signature, secret) {
  // Check timestamp (5 minutes tolerance)
  if (Math.abs(Date.now() / 1000 - parseInt(timestamp)) > 300) {
    return false;
  }
  
  // Calculate expected signature
  const message = `${timestamp}.${body}`;
  const expected = crypto
    .createHmac('sha256', secret)
    .update(message)
    .digest('hex');
  
  return crypto.timingSafeEqual(
    Buffer.from(expected),
    Buffer.from(signature)
  );
}
```

---

### GET /api/v1/webhooks/events
Liệt kê webhook events.

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| status | string | all | Filter by status: pending, processed, failed |
| limit | int | 50 | Số events tối đa |

**Response:**
```json
{
  "events": [
    {
      "id": 1,
      "event_id": "abc123...",
      "event_type": "ORDER_STATUS_CHANGE",
      "shop_id": 1,
      "received_at": "2024-01-01T00:00:00Z",
      "process_status": "processed",
      "processed_at": "2024-01-01T00:00:01Z",
      "signature_valid": true,
      "error": ""
    }
  ],
  "count": 1
}
```

---

### POST /api/v1/webhooks/events/:id/retry
Retry xử lý một webhook event failed.

**Path Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| id | int | Event ID (not event_id) |

**Response:**
```json
{
  "message": "event requeued",
  "event_id": "abc123..."
}
```

---

## 📦 TikTok API Client (Internal)

Các API client nội bộ để gọi TikTok Shop API:

### Orders API (`internal/tiktok/orders.go`)

| Method | Description |
|--------|-------------|
| `GetOrderList(ctx, req)` | Lấy danh sách đơn hàng |
| `GetOrderDetail(ctx, shopID, shopCipher, orderID)` | Lấy chi tiết đơn hàng |
| `GetRecentOrders(ctx, shopID, shopCipher, since)` | Lấy đơn hàng gần đây |

**Order Statuses:**
| Status | Description |
|--------|-------------|
| UNPAID | Chưa thanh toán |
| ON_HOLD | Đã thanh toán, trong thời gian remorse (1h) |
| AWAITING_SHIPMENT | Chờ gửi hàng |
| AWAITING_COLLECTION | Đã ship, chờ carrier lấy |
| IN_TRANSIT | Đang vận chuyển |
| DELIVERED | Đã giao |
| COMPLETED | Hoàn thành |
| CANCELLED | Đã hủy |

---

### Products API (`internal/tiktok/products.go`)

| Method | Description |
|--------|-------------|
| `GetProductList(ctx, req)` | Lấy danh sách sản phẩm |
| `GetProductDetail(ctx, shopID, shopCipher, productID)` | Chi tiết sản phẩm |
| `UpdateInventory(ctx, shopID, shopCipher, req)` | Cập nhật tồn kho |
| `UpdatePrice(ctx, shopID, shopCipher, req)` | Cập nhật giá |
| `ActivateProducts(ctx, shopID, shopCipher, productIDs)` | Kích hoạt sản phẩm |
| `DeactivateProducts(ctx, shopID, shopCipher, productIDs)` | Ẩn sản phẩm |

**Example - Update Inventory:**
```go
err := productsAPI.UpdateInventory(ctx, shopID, shopCipher, &UpdateInventoryRequest{
    ProductID: "123456",
    SKUs: []UpdateSKUInventory{
        {
            ID: "sku_123",
            Inventory: []InventoryUpdate{
                {WarehouseID: "wh_001", Quantity: 10},
            },
        },
    },
})
```

---

### Fulfillment API (`internal/tiktok/fulfillment.go`)

| Method | Description |
|--------|-------------|
| `ShipPackage(ctx, shopID, shopCipher, req)` | Gửi tracking number |
| `GetPackageDetail(ctx, shopID, shopCipher, packageID)` | Chi tiết package |
| `MarkPackageDelivered(ctx, shopID, shopCipher, req)` | Đánh dấu đã giao (3PL) |

**Example - Ship Package:**
```go
resp, err := fulfillmentAPI.ShipPackage(ctx, shopID, shopCipher, &ShipPackageRequest{
    OrderID:            "order_123",
    TrackingNumber:     "VN123456789",
    ShippingProviderID: "provider_001",
})
```

---

### Logistics API (`internal/tiktok/logistics.go`)

| Method | Description |
|--------|-------------|
| `GetWarehouses(ctx, shopID, shopCipher)` | Lấy danh sách kho |
| `GetShippingProviders(ctx, shopID, shopCipher)` | Lấy nhà vận chuyển |
| `GetDeliveryOptions(ctx, shopID, shopCipher)` | Lấy options giao hàng |

---

### Auth API (`internal/tiktok/auth.go`)

| Method | Description |
|--------|-------------|
| `GetAccessToken(ctx, authCode)` | Exchange auth code → tokens |
| `RefreshAccessToken(ctx, refreshToken)` | Refresh access token |
| `GetValidToken(ctx, shopID)` | Lấy token hợp lệ (auto refresh) |
| `SaveToken(ctx, shopID, tokenResp)` | Lưu token vào DB |

---

## 🔒 Signature Generation

### API Request Signature
```
sign = HMAC-SHA256(
    app_secret + path + sorted_params + body + app_secret,
    app_secret
)
```

**Steps:**
1. Lấy tất cả query params (trừ `sign` và `access_token`)
2. Sắp xếp params theo alphabet
3. Nối: `key1value1key2value2...`
4. Tạo string: `{app_secret}{path}{params}{body}{app_secret}`
5. HMAC-SHA256 với app_secret

### Webhook Signature Verification
```
Header: Tiktok-Signature: t=1633174587,s=18494715036ac4416a...

signed_payload = timestamp + "." + request_body
expected_sig = HMAC-SHA256(signed_payload, client_secret)
```

---

## ⚠️ Error Codes

| Code | Message | Description |
|------|---------|-------------|
| 0 | success | Thành công |
| 105001 | Invalid sign | Signature không đúng |
| 105002 | Timestamp expired | Timestamp quá 5 phút |
| 105003 | Invalid app_key | App key không tồn tại |
| 105004 | Invalid access_token | Token hết hạn hoặc sai |
| 130001 | Shop not found | Shop không tồn tại |
| 130002 | Shop not authorized | Shop chưa authorize |

---

## 📝 Rate Limits

- Default: 10 requests/second per app
- Bulk operations: 1 request/second
- Auto retry với exponential backoff khi gặp 429
