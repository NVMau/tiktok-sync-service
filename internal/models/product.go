package models

import (
	"time"

	"gorm.io/datatypes"
)

// =============================================================================
// PRODUCT - Product on TikTok Shop (Phone number group)
// =============================================================================
// Each Product on TikTok corresponds to a number group (e.g., "Lucky Number SIM", "Fortune SIM")
// A Product contains multiple SKUs, each SKU is a specific phone number
//
// Flow:
//   1. Shop is authorized → Sync all Products from TikTok to DB
//   2. Each Product has multiple SKUs (phone numbers)
//   3. External system sends new SKU → Add to corresponding Product → Push to TikTok

// ProductStatus - Product status on TikTok
type ProductStatus string

const (
	// ProductStatusActive - Product is active, available for sale
	ProductStatusActive ProductStatus = "ACTIVE"

	// ProductStatusInactive - Product is disabled, not displayed on shop
	ProductStatusInactive ProductStatus = "INACTIVE"

	// ProductStatusDraft - Draft product, not yet published
	ProductStatusDraft ProductStatus = "DRAFT"

	// ProductStatusDeleted - Product has been deleted on TikTok
	ProductStatusDeleted ProductStatus = "DELETED"
)

type Product struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ShopID          uint           `gorm:"index;not null" json:"shop_id"`                          // FK → shops.id
	TikTokShopID    string         `gorm:"size:100;index;not null" json:"tiktok_shop_id"`          // Actual TikTok shop ID (for sync)
	TikTokProductID string         `gorm:"size:100;uniqueIndex;not null" json:"tiktok_product_id"` // ID from TikTok
	Title           string         `gorm:"size:500;not null" json:"title"`                         // "Lucky Number SIM", "Fortune SIM"
	Description     string         `gorm:"type:text" json:"description"`                           // Product description
	CategoryID      string         `gorm:"size:100" json:"category_id"`                            // Category on TikTok
	Status          ProductStatus  `gorm:"size:50;index;default:ACTIVE" json:"status"`             // Product status
	SKUCount        int            `gorm:"default:0" json:"sku_count"`                             // Number of SKUs in product
	MainImages      datatypes.JSON `gorm:"type:jsonb" json:"main_images"`                          // Product images [{uri, width, height}]
	RawData         datatypes.JSON `gorm:"type:jsonb" json:"raw_data"`                             // Raw response from TikTok API
	SyncedAt        *time.Time     `json:"synced_at"`                                              // Last sync time from TikTok
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`

	// Relations (not migrated, used for preload only)
	Shop *Shop `gorm:"-" json:"shop,omitempty"`
	SKUs []SKU `gorm:"-" json:"skus,omitempty"`
}

func (Product) TableName() string {
	return "tiktok_sync.products"
}

// =============================================================================
// SKU - Each SKU = 1 specific phone number
// =============================================================================
// SKU is the smallest sellable unit. In the premium SIM number business:
//   - Each phone number is a separate SKU
//   - Quantity is always 0 or 1 (because phone numbers are unique)
//   - seller_sku is the phone number itself: "0912345678"
//
// Sync flow:
//   1. Shop auth → Sync existing SKUs from TikTok (sync_status = SYNCED)
//   2. External system calls API to add new SKU (sync_status = PENDING)
//   3. Worker picks up → Push to TikTok
//   4. Success → sync_status = PUSHED, save tiktok_sku_id
//   5. Failure → sync_status = FAILED, save error_message
//
// Sales flow:
//   1. New order placed → sale_status = RESERVED
//   2. Order completed → sale_status = SOLD, quantity = 0
//   3. Order cancelled → sale_status = AVAILABLE, quantity = 1

// SKUSyncStatus - SKU sync status with TikTok
type SKUSyncStatus string

const (
	// SKUSyncStatusSynced - SKU already exists on TikTok, synced to DB
	// This is a SKU that existed on TikTok before connecting to the system
	SKUSyncStatusSynced SKUSyncStatus = "SYNCED"

	// SKUSyncStatusPending - New SKU added from external system, waiting to push to TikTok
	// Worker will pick up these SKUs and push to TikTok
	SKUSyncStatusPending SKUSyncStatus = "PENDING"

	// SKUSyncStatusPushing - SKU is being pushed to TikTok
	// Prevents duplicate push when there are multiple workers
	SKUSyncStatusPushing SKUSyncStatus = "PUSHING"

	// SKUSyncStatusPushed - SKU has been successfully pushed to TikTok
	// tiktok_sku_id will be updated after successful push
	SKUSyncStatusPushed SKUSyncStatus = "PUSHED"

	// SKUSyncStatusFailed - Push to TikTok failed
	// Error details are stored in error_message
	// Can retry by changing back to PENDING
	SKUSyncStatusFailed SKUSyncStatus = "FAILED"
)

// SKUSaleStatus - SKU sales status
type SKUSaleStatus string

const (
	// SKUSaleStatusAvailable - Number is ready for sale
	// quantity = 1, can receive new orders
	SKUSaleStatusAvailable SKUSaleStatus = "AVAILABLE"

	// SKUSaleStatusReserved - Number has an order, pending processing
	// When a new order contains this SKU, change to RESERVED
	// Prevents double selling
	SKUSaleStatusReserved SKUSaleStatus = "RESERVED"

	// SKUSaleStatusSold - Number has been sold successfully
	// quantity = 0, cannot be resold
	// Only change when order is COMPLETED
	SKUSaleStatusSold SKUSaleStatus = "SOLD"

	// SKUSaleStatusUnavailable - Number is not available (out of stock, error, etc.)
	// Used when temporarily hiding number without deleting
	SKUSaleStatusUnavailable SKUSaleStatus = "UNAVAILABLE"
)

type SKU struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	ProductID     uint           `gorm:"index;not null" json:"product_id"`                              // FK → products.id
	TikTokSKUID   *string        `gorm:"column:tik_tok_sku_id;size:100;uniqueIndex" json:"tiktok_sku_id"` // ID from TikTok (UNIQUE)
	SellerSKU     string         `gorm:"size:100;index;not null" json:"seller_sku"`                     // Phone number (not unique as it can repeat across products)
	Price         float64        `gorm:"type:decimal(15,2);not null" json:"price"`           // Sale price (VND)
	OriginalPrice *float64       `gorm:"type:decimal(15,2)" json:"original_price,omitempty"` // Original price (if discounted)
	Quantity      int            `gorm:"not null;default:1" json:"quantity"`                 // 0 or 1

	// Sync status
	SyncStatus   SKUSyncStatus `gorm:"size:50;index;not null;default:PENDING" json:"sync_status"` // Sync status with TikTok
	SaleStatus   SKUSaleStatus `gorm:"size:50;index;not null;default:AVAILABLE" json:"sale_status"` // Sales status
	ErrorMessage *string       `gorm:"type:text" json:"error_message,omitempty"`                  // Error details if push failed
	PushAttempts int           `gorm:"default:0" json:"push_attempts"`                            // Number of push attempts

	// Variant attributes (if product has multiple variants like color, size)
	SalesAttributes datatypes.JSON `gorm:"type:jsonb" json:"sales_attributes,omitempty"` // [{id, name, value_id, value_name}]

	// Inventory info from TikTok
	InventoryInfo datatypes.JSON `gorm:"type:jsonb" json:"inventory_info,omitempty"` // [{warehouse_id, quantity}]

	// Timestamps
	SyncedAt  *time.Time `json:"synced_at,omitempty"` // Last sync time
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Relations (not migrated, used for preload only)
	Product *Product `gorm:"-" json:"product,omitempty"`
}

func (SKU) TableName() string {
	return "tiktok_sync.skus"
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// IsPushable checks if SKU can be pushed to TikTok
func (s *SKU) IsPushable() bool {
	return s.SyncStatus == SKUSyncStatusPending || s.SyncStatus == SKUSyncStatusFailed
}

// IsSellable checks if SKU can be sold
func (s *SKU) IsSellable() bool {
	return s.SaleStatus == SKUSaleStatusAvailable && s.Quantity > 0
}

// MarkAsPushing marks SKU as being pushed
func (s *SKU) MarkAsPushing() {
	s.SyncStatus = SKUSyncStatusPushing
	s.PushAttempts++
}

// MarkAsPushed marks SKU as successfully pushed
func (s *SKU) MarkAsPushed(tiktokSKUID string) {
	s.SyncStatus = SKUSyncStatusPushed
	s.TikTokSKUID = &tiktokSKUID
	s.ErrorMessage = nil
	now := time.Now()
	s.SyncedAt = &now
}

// MarkAsFailed marks SKU push as failed
func (s *SKU) MarkAsFailed(errMsg string) {
	s.SyncStatus = SKUSyncStatusFailed
	s.ErrorMessage = &errMsg
}

// Reserve marks SKU as having an order
func (s *SKU) Reserve() {
	s.SaleStatus = SKUSaleStatusReserved
}

// MarkAsSold marks SKU as sold
func (s *SKU) MarkAsSold() {
	s.SaleStatus = SKUSaleStatusSold
	s.Quantity = 0
}

// Release releases SKU (when order is cancelled)
func (s *SKU) Release() {
	s.SaleStatus = SKUSaleStatusAvailable
	s.Quantity = 1
}
