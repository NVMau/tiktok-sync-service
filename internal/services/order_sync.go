package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
)

// =============================================================================
// OrderSyncService - Manages order synchronization
// =============================================================================
// Order processing flow:
//   1. TikTok sends webhook ORDER_CREATED or ORDER_STATUS_CHANGE
//   2. Save order to DB (sync_state = new)
//   3. Map order_items with SKUs in DB
//   4. Reserve SKU when order is placed (prevent double selling)
//   5. Mark SKU as sold when order is COMPLETED
//   6. Release SKU when order is CANCELLED

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

// MapTikTokStatusToLocal - Convert TikTok status to local status
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

// SyncOrderToLocal - Sync order from TikTok to local system
func (s *OrderSyncService) SyncOrderToLocal(ctx context.Context, order *models.Order) error {
	logger.Info("syncing order to local", zap.String("tiktok_order_id", order.TikTokOrderID))

	// Get order items by TikTok order ID
	var items []models.OrderItem
	if err := database.DB.Where("tik_tok_order_id = ?", order.TikTokOrderID).Find(&items).Error; err != nil {
		return fmt.Errorf("failed to get order items: %w", err)
	}

	// Lookup SKU for each item and perform actions
	for _, item := range items {
		tiktokSKUID := item.TikTokSKUID
		sellerSKU := item.SellerSKU

		// Try to find SKU by TikTokSKUID or SellerSKU
		sku, err := s.lookupSKU(ctx, tiktokSKUID, sellerSKU)
		if err != nil {
			logger.Warn("failed to lookup SKU for order item",
				zap.String("item_id", item.TikTokOrderItemID),
				zap.String("tiktok_sku_id", tiktokSKUID),
				zap.String("seller_sku", sellerSKU),
				zap.Error(err))
			order.SyncState = models.SyncStateManualReview
			order.RawPayload = appendError(order.RawPayload, fmt.Sprintf("item %s: %v", item.TikTokOrderItemID, err))
			continue
		}

		// Get TikTokSKUID from found SKU (required for operations)
		skuTikTokID := ""
		if sku.TikTokSKUID != nil {
			skuTikTokID = *sku.TikTokSKUID
		}
		if skuTikTokID == "" {
			logger.Warn("SKU has no tik_tok_sku_id, skipping operations",
				zap.String("seller_sku", sku.SellerSKU),
				zap.Uint("sku_id", sku.ID))
			continue
		}

		// Reserve SKUs when order is paid
		if s.shouldReserveSKU(order.TikTokOrderStatus) {
			if err := s.reserveSKU(ctx, skuTikTokID, order.TikTokOrderID); err != nil {
				logger.Warn("failed to reserve SKU",
					zap.String("tiktok_sku_id", skuTikTokID),
					zap.Error(err))
				order.SyncState = models.SyncStateManualReview
			}
		}

		// Mark SKUs as sold when order is completed
		if order.TikTokOrderStatus == models.OrderStatusCompleted {
			if err := s.markSKUAsSold(ctx, skuTikTokID); err != nil {
				logger.Error("failed to mark SKU as sold",
					zap.String("tiktok_sku_id", skuTikTokID),
					zap.Error(err))
			}
		}

		// Release SKUs when order is cancelled
		if order.TikTokOrderStatus == models.OrderStatusCancelled {
			if err := s.releaseSKU(ctx, skuTikTokID); err != nil {
				logger.Error("failed to release SKU",
					zap.String("tiktok_sku_id", skuTikTokID),
					zap.Error(err))
			}
		}
	}

	localStatus := s.MapTikTokStatusToLocal(order.TikTokOrderStatus)

	// Update sync state
	if order.SyncState != models.SyncStateManualReview {
		order.SyncState = models.SyncStateSynced
	}

	if err := database.DB.Save(order).Error; err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	logger.Info("order synced",
		zap.String("tiktok_order_id", order.TikTokOrderID),
		zap.String("tiktok_status", string(order.TikTokOrderStatus)),
		zap.String("local_status", string(localStatus)),
		zap.String("sync_state", string(order.SyncState)))

	return nil
}

// lookupSKU - Lookup SKU by TikTokSKUID or SellerSKU
// Priority lookup by tik_tok_sku_id (most accurate), fallback to seller_sku
func (s *OrderSyncService) lookupSKU(ctx context.Context, tiktokSKUID, sellerSKU string) (*models.SKU, error) {
	var sku models.SKU

	// Priority 1: Lookup by tik_tok_sku_id (100% accurate from TikTok)
	if tiktokSKUID != "" {
		err := database.DB.Where("tik_tok_sku_id = ?", tiktokSKUID).First(&sku).Error
		if err == nil {
			return &sku, nil
		}
	}

	// Priority 2: Lookup by seller_sku (phone number)
	if sellerSKU != "" {
		err := database.DB.Where("seller_sku = ?", sellerSKU).First(&sku).Error
		if err == nil {
			return &sku, nil
		}
	}

	// Both failed
	if tiktokSKUID == "" && sellerSKU == "" {
		return nil, fmt.Errorf("no tik_tok_sku_id or seller_sku to lookup")
	}
	return nil, fmt.Errorf("no SKU found for tik_tok_sku_id=%s seller_sku=%s", tiktokSKUID, sellerSKU)
}

// shouldReserveSKU - Check if SKU should be reserved
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

// reserveSKU - Reserve SKU (with transaction lock to prevent race condition)
func (s *OrderSyncService) reserveSKU(ctx context.Context, tiktokSKUID, orderRef string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sku models.SKU
	err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("tik_tok_sku_id = ?", tiktokSKUID).
		First(&sku).Error

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("SKU tik_tok_sku_id=%s not found: %w", tiktokSKUID, err)
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

	logger.Info("reserved SKU for order",
		zap.String("seller_sku", sku.SellerSKU),
		zap.String("tiktok_sku_id", tiktokSKUID),
		zap.String("order_ref", orderRef))
	return nil
}

// markSKUAsSold - Mark SKU as sold
func (s *OrderSyncService) markSKUAsSold(ctx context.Context, tiktokSKUID string) error {
	return database.DB.Model(&models.SKU{}).
		Where("tik_tok_sku_id = ?", tiktokSKUID).
		Updates(map[string]interface{}{
			"quantity":    0,
			"sale_status": models.SKUSaleStatusSold,
			"updated_at":  time.Now(),
		}).Error
}

// releaseSKU - Release SKU when order is cancelled
func (s *OrderSyncService) releaseSKU(ctx context.Context, tiktokSKUID string) error {
	return database.DB.Model(&models.SKU{}).
		Where("tik_tok_sku_id = ?", tiktokSKUID).
		Updates(map[string]interface{}{
			"quantity":    1,
			"sale_status": models.SKUSaleStatusAvailable,
			"updated_at":  time.Now(),
		}).Error
}

// =============================================================================
// QUERY METHODS
// =============================================================================

// GetPendingSyncOrders - Get orders that have not been synced
func (s *OrderSyncService) GetPendingSyncOrders(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("sync_state = ?", models.SyncStateNew).
		Order("created_at ASC").
		Limit(100).
		Find(&orders).Error

	return orders, err
}

// GetManualReviewOrders - Get orders that need manual review
func (s *OrderSyncService) GetManualReviewOrders(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("sync_state = ?", models.SyncStateManualReview).
		Order("created_at ASC").
		Find(&orders).Error

	return orders, err
}

// ReleaseSKU - Public method to release SKU
func (s *OrderSyncService) ReleaseSKU(ctx context.Context, tiktokSKUID, orderRef string) error {
	if err := s.releaseSKU(ctx, tiktokSKUID); err != nil {
		return err
	}
	logger.Info("released SKU from order",
		zap.String("tiktok_sku_id", tiktokSKUID),
		zap.String("order_ref", orderRef))
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
