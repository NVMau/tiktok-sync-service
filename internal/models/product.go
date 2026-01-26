package models

import (
	"time"

	"gorm.io/datatypes"
)

// =============================================================================
// PRODUCT - Sản phẩm trên TikTok Shop (Nhóm số điện thoại)
// =============================================================================
// Mỗi Product trên TikTok tương ứng với 1 nhóm số (VD: "Sim Thần Tài", "Sim Lộc Phát")
// Một Product chứa nhiều SKU, mỗi SKU là 1 số điện thoại cụ thể
//
// Flow:
//   1. Shop được authorize → Sync tất cả Products từ TikTok về DB
//   2. Mỗi Product có nhiều SKUs (số điện thoại)
//   3. External system gửi SKU mới → Thêm vào Product tương ứng → Push lên TikTok

// ProductStatus - Trạng thái của Product trên TikTok
type ProductStatus string

const (
	// ProductStatusActive - Sản phẩm đang hoạt động, có thể bán
	ProductStatusActive ProductStatus = "ACTIVE"

	// ProductStatusInactive - Sản phẩm bị tắt, không hiển thị trên shop
	ProductStatusInactive ProductStatus = "INACTIVE"

	// ProductStatusDraft - Sản phẩm nháp, chưa publish
	ProductStatusDraft ProductStatus = "DRAFT"

	// ProductStatusDeleted - Sản phẩm đã bị xóa trên TikTok
	ProductStatusDeleted ProductStatus = "DELETED"
)

type Product struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ShopID          uint           `gorm:"index;not null" json:"shop_id"`                          // FK → shops.id
	TikTokProductID string         `gorm:"size:100;uniqueIndex;not null" json:"tiktok_product_id"` // ID từ TikTok
	Title           string         `gorm:"size:500;not null" json:"title"`                         // "Sim Thần Tài", "Sim Lộc Phát"
	Description     string         `gorm:"type:text" json:"description"`                           // Mô tả sản phẩm
	CategoryID      string         `gorm:"size:100" json:"category_id"`                            // Category trên TikTok
	Status          ProductStatus  `gorm:"size:50;index;default:ACTIVE" json:"status"`             // Trạng thái product
	SKUCount        int            `gorm:"default:0" json:"sku_count"`                             // Số lượng SKU trong product
	MainImages      datatypes.JSON `gorm:"type:jsonb" json:"main_images"`                          // Ảnh sản phẩm [{uri, width, height}]
	RawData         datatypes.JSON `gorm:"type:jsonb" json:"raw_data"`                             // Response gốc từ TikTok API
	SyncedAt        *time.Time     `json:"synced_at"`                                              // Lần sync cuối từ TikTok
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`

	// Relations
	Shop *Shop `gorm:"foreignKey:ShopID" json:"shop,omitempty"`
	SKUs []SKU `gorm:"foreignKey:ProductID" json:"skus,omitempty"`
}

func (Product) TableName() string {
	return "tiktok_sync.products"
}

// =============================================================================
// SKU - Mỗi SKU = 1 số điện thoại cụ thể
// =============================================================================
// SKU là đơn vị nhỏ nhất có thể bán. Trong business SIM số đẹp:
//   - Mỗi số điện thoại là 1 SKU riêng biệt
//   - Quantity luôn là 0 hoặc 1 (vì số điện thoại là unique)
//   - seller_sku chính là số điện thoại: "0912345678"
//
// Flow sync:
//   1. Shop auth → Sync SKUs có sẵn từ TikTok (sync_status = SYNCED)
//   2. External system gọi API thêm SKU mới (sync_status = PENDING)
//   3. Worker pick up → Push lên TikTok
//   4. Thành công → sync_status = PUSHED, lưu tiktok_sku_id
//   5. Thất bại → sync_status = FAILED, lưu error_message
//
// Flow bán hàng:
//   1. Có đơn hàng mới → sale_status = RESERVED
//   2. Đơn hoàn thành → sale_status = SOLD, quantity = 0
//   3. Đơn bị hủy → sale_status = AVAILABLE, quantity = 1

// SKUSyncStatus - Trạng thái đồng bộ SKU với TikTok
type SKUSyncStatus string

const (
	// SKUSyncStatusSynced - SKU đã có trên TikTok, được sync về DB
	// Đây là SKU đã tồn tại trên TikTok trước khi connect với hệ thống
	SKUSyncStatusSynced SKUSyncStatus = "SYNCED"

	// SKUSyncStatusPending - SKU mới được thêm từ external system, chờ push lên TikTok
	// Worker sẽ pick up các SKU này và push lên TikTok
	SKUSyncStatusPending SKUSyncStatus = "PENDING"

	// SKUSyncStatusPushing - SKU đang được push lên TikTok
	// Tránh duplicate push khi có nhiều worker
	SKUSyncStatusPushing SKUSyncStatus = "PUSHING"

	// SKUSyncStatusPushed - SKU đã push lên TikTok thành công
	// tiktok_sku_id sẽ được cập nhật sau khi push thành công
	SKUSyncStatusPushed SKUSyncStatus = "PUSHED"

	// SKUSyncStatusFailed - Push lên TikTok thất bại
	// Chi tiết lỗi được lưu trong error_message
	// Có thể retry bằng cách chuyển về PENDING
	SKUSyncStatusFailed SKUSyncStatus = "FAILED"
)

// SKUSaleStatus - Trạng thái bán hàng của SKU
type SKUSaleStatus string

const (
	// SKUSaleStatusAvailable - Số đang sẵn sàng để bán
	// quantity = 1, có thể nhận đơn hàng mới
	SKUSaleStatusAvailable SKUSaleStatus = "AVAILABLE"

	// SKUSaleStatusReserved - Số đã có đơn hàng, đang chờ xử lý
	// Khi có đơn hàng mới chứa SKU này, chuyển sang RESERVED
	// Tránh bán trùng số (double selling)
	SKUSaleStatusReserved SKUSaleStatus = "RESERVED"

	// SKUSaleStatusSold - Số đã bán thành công
	// quantity = 0, không thể bán lại
	// Chỉ chuyển sang khi đơn hàng COMPLETED
	SKUSaleStatusSold SKUSaleStatus = "SOLD"

	// SKUSaleStatusUnavailable - Số không khả dụng (hết hàng, lỗi, etc.)
	// Dùng khi cần tạm ẩn số mà không xóa
	SKUSaleStatusUnavailable SKUSaleStatus = "UNAVAILABLE"
)

type SKU struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	ProductID     uint           `gorm:"index;not null" json:"product_id"`                    // FK → products.id
	TikTokSKUID   *string        `gorm:"size:100;index" json:"tiktok_sku_id"`                 // ID từ TikTok (null nếu chưa push)
	SellerSKU     string         `gorm:"size:100;uniqueIndex;not null" json:"seller_sku"`    // Số điện thoại: "0912345678"
	Price         float64        `gorm:"type:decimal(15,2);not null" json:"price"`           // Giá bán (VND)
	OriginalPrice *float64       `gorm:"type:decimal(15,2)" json:"original_price,omitempty"` // Giá gốc (nếu có giảm giá)
	Quantity      int            `gorm:"not null;default:1" json:"quantity"`                 // 0 hoặc 1

	// Sync status
	SyncStatus   SKUSyncStatus `gorm:"size:50;index;not null;default:PENDING" json:"sync_status"` // Trạng thái sync với TikTok
	SaleStatus   SKUSaleStatus `gorm:"size:50;index;not null;default:AVAILABLE" json:"sale_status"` // Trạng thái bán hàng
	ErrorMessage *string       `gorm:"type:text" json:"error_message,omitempty"`                  // Chi tiết lỗi nếu push failed
	PushAttempts int           `gorm:"default:0" json:"push_attempts"`                            // Số lần thử push

	// Variant attributes (nếu product có nhiều variant như màu, size)
	SalesAttributes datatypes.JSON `gorm:"type:jsonb" json:"sales_attributes,omitempty"` // [{id, name, value_id, value_name}]

	// Inventory info từ TikTok
	InventoryInfo datatypes.JSON `gorm:"type:jsonb" json:"inventory_info,omitempty"` // [{warehouse_id, quantity}]

	// Timestamps
	SyncedAt  *time.Time `json:"synced_at,omitempty"` // Lần sync cuối
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Relations
	Product *Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}

func (SKU) TableName() string {
	return "tiktok_sync.skus"
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// IsPushable kiểm tra SKU có thể push lên TikTok không
func (s *SKU) IsPushable() bool {
	return s.SyncStatus == SKUSyncStatusPending || s.SyncStatus == SKUSyncStatusFailed
}

// IsSellable kiểm tra SKU có thể bán không
func (s *SKU) IsSellable() bool {
	return s.SaleStatus == SKUSaleStatusAvailable && s.Quantity > 0
}

// MarkAsPushing đánh dấu SKU đang được push
func (s *SKU) MarkAsPushing() {
	s.SyncStatus = SKUSyncStatusPushing
	s.PushAttempts++
}

// MarkAsPushed đánh dấu SKU đã push thành công
func (s *SKU) MarkAsPushed(tiktokSKUID string) {
	s.SyncStatus = SKUSyncStatusPushed
	s.TikTokSKUID = &tiktokSKUID
	s.ErrorMessage = nil
	now := time.Now()
	s.SyncedAt = &now
}

// MarkAsFailed đánh dấu SKU push thất bại
func (s *SKU) MarkAsFailed(errMsg string) {
	s.SyncStatus = SKUSyncStatusFailed
	s.ErrorMessage = &errMsg
}

// Reserve đánh dấu SKU đã có đơn hàng
func (s *SKU) Reserve() {
	s.SaleStatus = SKUSaleStatusReserved
}

// MarkAsSold đánh dấu SKU đã bán
func (s *SKU) MarkAsSold() {
	s.SaleStatus = SKUSaleStatusSold
	s.Quantity = 0
}

// Release giải phóng SKU (khi đơn bị hủy)
func (s *SKU) Release() {
	s.SaleStatus = SKUSaleStatusAvailable
	s.Quantity = 1
}
