package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/models"
)

// =============================================================================
// OrderSyncService - Quản lý đồng bộ đơn hàng
// =============================================================================
// Flow xử lý đơn hàng:
//   1. TikTok gửi webhook ORDER_CREATED hoặc ORDER_STATUS_CHANGE
//   2. Lưu order vào DB (sync_state = new)
//   3. Map order_items với SKUs trong DB
//   4. Reserve SKU khi có đơn hàng (tránh bán trùng)
//   5. Mark SKU as sold khi đơn COMPLETED
//   6. Release SKU khi đơn bị CANCELLED

type LocalOrderStatus string

const (
	LocalStatusPendingPayment        LocalOrderStatus = "PENDING_PAYMENT"
	LocalStatusPaidHold              LocalOrderStatus = "PAID_HOLD"
	LocalStatusReadyToShip           LocalOrderStatus = "READY_TO_SHIP"
	LocalStatusShippedAwaitingPickup LocalOrderStatus = "SHIPPED_AWAITING_PICKUP"
	LocalStatusInTransit             LocalOrderStatus = "IN_TRANSIT"
	LocalStatusDelivered             LocalOrderStatus = "DELIVERED"
	LocalStatusCompleted             LocalOrderStatus = "COMPLETED"
	LocalStatusCancelled             LocalOrderStatus = "CANCELLED"
)

type OrderSyncService struct{}

func NewOrderSyncService() *OrderSyncService {
	return &OrderSyncService{}
}

// MapTikTokStatusToLocal - Chuyển đổi trạng thái TikTok sang local
func (s *OrderSyncService) MapTikTokStatusToLocal(tiktokStatus models.TikTokOrderStatus) LocalOrderStatus {
	switch tiktokStatus {
	case models.OrderStatusUnpaid:
		return LocalStatusPendingPayment
	case models.OrderStatusOnHold:
		return LocalStatusPaidHold
	case models.OrderStatusAwaitingShipment:
		return LocalStatusReadyToShip
	case models.OrderStatusPartiallyShipping, models.OrderStatusAwaitingCollection:
		return LocalStatusShippedAwaitingPickup
	case models.OrderStatusInTransit:
		return LocalStatusInTransit
	case models.OrderStatusDelivered:
		return LocalStatusDelivered
	case models.OrderStatusCompleted:
		return LocalStatusCompleted
	case models.OrderStatusCancelled:
		return LocalStatusCancelled
	default:
		return LocalStatusPendingPayment
	}
}

// =============================================================================
// SYNC ORDER
// =============================================================================

// SyncOrderToLocal - Đồng bộ order từ TikTok vào hệ thống local
func (s *OrderSyncService) SyncOrderToLocal(ctx context.Context, order *models.Order) error {
	log.Printf("Syncing order %s to local", order.TikTokOrderID)

	// Get order items
	var items []models.OrderItem
	if err := database.DB.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
		return fmt.Errorf("failed to get order items: %w", err)
	}

	// Map each item to SKU
	for i := range items {
		if items[i].SKUID != nil {
			continue // Already mapped
		}

		if err := s.mapOrderItemToSKU(ctx, &items[i]); err != nil {
			log.Printf("Failed to map item %s to SKU: %v", items[i].TikTokOrderItemID, err)
			order.SyncState = models.SyncStateManualReview
			order.RawPayload = appendError(order.RawPayload, fmt.Sprintf("item %s: %v", items[i].TikTokOrderItemID, err))
		}
	}

	localStatus := s.MapTikTokStatusToLocal(order.TikTokOrderStatus)

	// Reserve SKUs when order is paid
	if s.shouldReserveSKU(order.TikTokOrderStatus) {
		for _, item := range items {
			if item.SKUID == nil {
				continue
			}
			if err := s.reserveSKU(ctx, *item.SKUID, order.TikTokOrderID); err != nil {
				log.Printf("Failed to reserve SKU %d: %v", *item.SKUID, err)
				order.SyncState = models.SyncStateManualReview
			}
		}
	}

	// Mark SKUs as sold when order is completed
	if order.TikTokOrderStatus == models.OrderStatusCompleted {
		for _, item := range items {
			if item.SKUID == nil {
				continue
			}
			if err := s.markSKUAsSold(ctx, *item.SKUID); err != nil {
				log.Printf("Failed to mark SKU %d as sold: %v", *item.SKUID, err)
			}
		}
	}

	// Release SKUs when order is cancelled
	if order.TikTokOrderStatus == models.OrderStatusCancelled {
		for _, item := range items {
			if item.SKUID == nil {
				continue
			}
			if err := s.releaseSKU(ctx, *item.SKUID); err != nil {
				log.Printf("Failed to release SKU %d: %v", *item.SKUID, err)
			}
		}
	}

	// Update sync state
	if order.SyncState != models.SyncStateManualReview {
		order.SyncState = models.SyncStateSynced
	}

	if err := database.DB.Save(order).Error; err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	log.Printf("Order %s synced: tiktok_status=%s, local_status=%s, sync_state=%s",
		order.TikTokOrderID, order.TikTokOrderStatus, localStatus, order.SyncState)

	return nil
}

// mapOrderItemToSKU - Map order item với SKU trong DB dựa trên seller_sku
func (s *OrderSyncService) mapOrderItemToSKU(ctx context.Context, item *models.OrderItem) error {
	if item.SellerSKU == "" {
		return fmt.Errorf("no seller_sku")
	}

	// Find SKU by seller_sku (số điện thoại) or tiktok_sku_id
	var sku models.SKU
	err := database.DB.Where("seller_sku = ?", item.SellerSKU).
		Or("tiktok_sku_id = ?", item.TikTokSKUID).
		First(&sku).Error

	if err != nil {
		return fmt.Errorf("no SKU found for seller_sku %s", item.SellerSKU)
	}

	item.SKUID = &sku.ID
	if err := database.DB.Save(item).Error; err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	log.Printf("Mapped order item %s to SKU %d (seller_sku: %s)", item.TikTokOrderItemID, sku.ID, sku.SellerSKU)
	return nil
}

// shouldReserveSKU - Kiểm tra có nên reserve SKU không
func (s *OrderSyncService) shouldReserveSKU(status models.TikTokOrderStatus) bool {
	switch status {
	case models.OrderStatusOnHold,
		models.OrderStatusAwaitingShipment,
		models.OrderStatusPartiallyShipping,
		models.OrderStatusAwaitingCollection,
		models.OrderStatusInTransit,
		models.OrderStatusDelivered:
		return true
	default:
		return false
	}
}

// reserveSKU - Đặt trước SKU (với transaction lock để tránh race condition)
func (s *OrderSyncService) reserveSKU(ctx context.Context, skuID uint, orderRef string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sku models.SKU
	err := tx.Set("gorm:query_option", "FOR UPDATE").
		First(&sku, skuID).Error

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("SKU %d not found: %w", skuID, err)
	}

	// Check if already reserved or sold
	if sku.SaleStatus != models.SKUSaleStatusAvailable {
		tx.Rollback()
		return fmt.Errorf("SKU %s already %s", sku.SellerSKU, sku.SaleStatus)
	}

	// Reserve
	sku.SaleStatus = models.SKUSaleStatusReserved
	sku.UpdatedAt = time.Now()

	if err := tx.Save(&sku).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update SKU: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	log.Printf("Reserved SKU %s (ID: %d) for order %s", sku.SellerSKU, skuID, orderRef)
	return nil
}

// markSKUAsSold - Đánh dấu SKU đã bán
func (s *OrderSyncService) markSKUAsSold(ctx context.Context, skuID uint) error {
	return database.DB.Model(&models.SKU{}).
		Where("id = ?", skuID).
		Updates(map[string]interface{}{
			"quantity":    0,
			"sale_status": models.SKUSaleStatusSold,
			"updated_at":  time.Now(),
		}).Error
}

// releaseSKU - Giải phóng SKU khi đơn bị hủy
func (s *OrderSyncService) releaseSKU(ctx context.Context, skuID uint) error {
	return database.DB.Model(&models.SKU{}).
		Where("id = ?", skuID).
		Updates(map[string]interface{}{
			"quantity":    1,
			"sale_status": models.SKUSaleStatusAvailable,
			"updated_at":  time.Now(),
		}).Error
}

// =============================================================================
// QUERY METHODS
// =============================================================================

// GetPendingSyncOrders - Lấy các order chưa được sync
func (s *OrderSyncService) GetPendingSyncOrders(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("sync_state = ?", models.SyncStateNew).
		Order("created_at ASC").
		Limit(100).
		Find(&orders).Error

	return orders, err
}

// GetManualReviewOrders - Lấy các order cần review thủ công
func (s *OrderSyncService) GetManualReviewOrders(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("sync_state = ?", models.SyncStateManualReview).
		Order("created_at ASC").
		Find(&orders).Error

	return orders, err
}

// ReleaseSKU - Public method để giải phóng SKU
func (s *OrderSyncService) ReleaseSKU(ctx context.Context, skuID uint, orderRef string) error {
	if err := s.releaseSKU(ctx, skuID); err != nil {
		return err
	}
	log.Printf("Released SKU %d from order %s", skuID, orderRef)
	return nil
}

// =============================================================================
// HELPERS
// =============================================================================

func appendError(payload []byte, errMsg string) []byte {
	var data map[string]interface{}
	json.Unmarshal(payload, &data)
	if data == nil {
		data = make(map[string]interface{})
	}

	errors, _ := data["sync_errors"].([]interface{})
	errors = append(errors, errMsg)
	data["sync_errors"] = errors

	result, _ := json.Marshal(data)
	return result
}
