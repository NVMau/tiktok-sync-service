package models

import (
	"time"

	"gorm.io/datatypes"
)

type TikTokOrderStatus string

const (
	OrderStatusUnpaid             TikTokOrderStatus = "UNPAID"
	OrderStatusOnHold             TikTokOrderStatus = "ON_HOLD"
	OrderStatusAwaitingShipment   TikTokOrderStatus = "AWAITING_SHIPMENT"
	OrderStatusPartiallyShipping  TikTokOrderStatus = "PARTIALLY_SHIPPING"
	OrderStatusAwaitingCollection TikTokOrderStatus = "AWAITING_COLLECTION"
	OrderStatusInTransit          TikTokOrderStatus = "IN_TRANSIT"
	OrderStatusDelivered          TikTokOrderStatus = "DELIVERED"
	OrderStatusCompleted          TikTokOrderStatus = "COMPLETED"
	OrderStatusCancelled          TikTokOrderStatus = "CANCELLED"
)

type SyncState string

const (
	SyncStateNew          SyncState = "new"
	SyncStateSynced       SyncState = "synced"
	SyncStateFailed       SyncState = "failed"
	SyncStateManualReview SyncState = "manual_review"
)

type Order struct {
	ID                uint              `gorm:"primaryKey" json:"id"`
	ShopID            uint              `gorm:"index;not null" json:"shop_id"`                                           // FK → shops.id
	TikTokOrderID     string            `gorm:"column:tik_tok_order_id;size:100;uniqueIndex;not null" json:"tiktok_order_id"`
	TikTokOrderStatus TikTokOrderStatus `gorm:"column:tik_tok_order_status;size:50;index" json:"tiktok_order_status"`
	PaymentStatus     string            `gorm:"size:50" json:"payment_status"`                                           // PAID / UNPAID
	BuyerInfo         datatypes.JSON    `gorm:"type:jsonb" json:"buyer_info"`                                            // {name, phone, email}
	ShippingAddress   datatypes.JSON    `gorm:"type:jsonb" json:"shipping_address"`                                      // Địa chỉ giao hàng
	TotalAmount       float64           `gorm:"type:decimal(15,2)" json:"total_amount"`
	Currency          string            `gorm:"size:10" json:"currency"`                                                 // VND
	PlacedAt          *time.Time        `json:"placed_at"`                                                               // Thời điểm đặt hàng
	PaidAt            *time.Time        `json:"paid_at"`                                                                 // Thời điểm thanh toán
	TrackingNumber    *string           `gorm:"size:100" json:"tracking_number,omitempty"`                               // Mã vận đơn
	ShippingProvider  *string           `gorm:"size:100" json:"shipping_provider,omitempty"`                             // GHTK, GHN...
	RawPayload        datatypes.JSON    `gorm:"type:jsonb" json:"raw_payload"`
	LocalOrderID      *string           `gorm:"size:50;index" json:"local_order_id"`                                     // ID nội bộ
	SyncState         SyncState         `gorm:"size:50;default:new;index" json:"sync_state"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`

	// Relations
	Shop  *Shop       `gorm:"foreignKey:ShopID" json:"shop,omitempty"`
	Items []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
}

func (Order) TableName() string {
	return "tiktok_sync.orders"
}

type OrderItem struct {
	ID                uint    `gorm:"primaryKey" json:"id"`
	OrderID           uint    `gorm:"index;not null" json:"order_id"`                                         // FK → orders.id
	SKUID             *uint   `gorm:"index" json:"sku_id"`                                                    // FK → skus.id (nullable)
	TikTokOrderItemID string  `gorm:"column:tik_tok_order_item_id;size:100;index" json:"tiktok_order_item_id"`
	TikTokProductID   string  `gorm:"column:tik_tok_product_id;size:100" json:"tiktok_product_id"`
	TikTokSKUID       string  `gorm:"column:tik_tok_sk_uid;size:100" json:"tiktok_sku_id"`
	SellerSKU         string  `gorm:"size:100;index" json:"seller_sku"`                                       // Số điện thoại
	Qty               int     `gorm:"not null;default:1" json:"qty"`
	Price             float64 `gorm:"type:decimal(15,2)" json:"price"`
	CreatedAt         time.Time `json:"created_at"`

	// Relations
	Order *Order `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	SKU   *SKU   `gorm:"foreignKey:SKUID" json:"sku,omitempty"`
}

func (OrderItem) TableName() string {
	return "tiktok_sync.order_items"
}
