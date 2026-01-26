package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/services"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

type WebhookTaskPayload struct {
	EventID   uint   `json:"event_id"`
	EventType string `json:"event_type"`
}

type OrderEventData struct {
	OrderID     string `json:"order_id"`
	OrderStatus string `json:"order_status"`
	UpdateTime  int64  `json:"update_time"`
	ShopCipher  string `json:"shop_cipher"`
}

func HandleProcessWebhook(ctx context.Context, task *asynq.Task) error {
	var payload WebhookTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to parse task payload: %w", err)
	}

	logger.Info("processing webhook event",
		zap.Uint("event_id", payload.EventID),
		zap.String("event_type", payload.EventType))

	var event models.WebhookEvent
	if err := database.DB.First(&event, payload.EventID).Error; err != nil {
		return fmt.Errorf("webhook event not found: %w", err)
	}

	if event.ProcessStatus == models.EventStatusProcessed {
		logger.Debug("event already processed, skipping", zap.Uint("event_id", payload.EventID))
		return nil
	}

	var processErr error
	switch payload.EventType {
	case "ORDER_STATUS_CHANGE", "ORDER_CREATED":
		processErr = processOrderEvent(ctx, &event)
	case "PACKAGE_UPDATE":
		processErr = processPackageEvent(ctx, &event)
	default:
		logger.Warn("unknown event type, marking as processed", zap.String("event_type", payload.EventType))
	}

	now := time.Now()
	if processErr != nil {
		event.ProcessStatus = models.EventStatusFailed
		event.Error = processErr.Error()
		event.ProcessedAt = &now
		database.DB.Save(&event)
		return processErr
	}

	event.ProcessStatus = models.EventStatusProcessed
	event.ProcessedAt = &now
	database.DB.Save(&event)

	logger.Info("webhook event processed successfully", zap.Uint("event_id", payload.EventID))
	return nil
}

func processOrderEvent(ctx context.Context, event *models.WebhookEvent) error {
	var webhookPayload struct {
		Type              int            `json:"type"`
		TTSNotificationID string         `json:"tts_notification_id"`
		ShopID            string         `json:"shop_id"`
		Timestamp         int64          `json:"timestamp"`
		Data              OrderEventData `json:"data"`
	}

	if err := json.Unmarshal(event.Payload, &webhookPayload); err != nil {
		return fmt.Errorf("failed to parse order event: %w", err)
	}

	data := webhookPayload.Data
	logger.Info("processing order event",
		zap.String("order_id", data.OrderID),
		zap.String("order_status", data.OrderStatus))

	var shop models.Shop
	if err := database.DB.First(&shop, event.ShopID).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	cfg := config.Get()
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)
	ordersAPI := tiktok.NewOrdersAPI(client, tokenManager)

	// Use shop_cipher from DB, not from webhook payload
	orderDetail, err := ordersAPI.GetOrderDetail(ctx, shop.ID, shop.ShopCipher, data.OrderID)
	if err != nil {
		return fmt.Errorf("failed to get order detail: %w", err)
	}

	return upsertOrder(shop.ID, shop.ShopID, orderDetail, event.Payload)
}

func upsertOrder(shopID uint, tiktokShopID string, detail *tiktok.OrderDetailResponse, rawPayload []byte) error {
	var placedAt, paidAt *time.Time
	if detail.CreateTime > 0 {
		t := time.Unix(detail.CreateTime, 0)
		placedAt = &t
	}
	if detail.PaidTime > 0 {
		t := time.Unix(detail.PaidTime, 0)
		paidAt = &t
	}

	totalAmount := 0.0
	if detail.Payment.TotalAmount != "" {
		fmt.Sscanf(detail.Payment.TotalAmount, "%f", &totalAmount)
	}

	buyerInfo, _ := json.Marshal(map[string]interface{}{
		"message": detail.BuyerMessage,
	})
	shippingAddr, _ := json.Marshal(detail.RecipientAddress)

	order := models.Order{
		ShopID:            shopID,
		TikTokShopID:      tiktokShopID,
		TikTokOrderID:     detail.ID,
		TikTokOrderStatus: models.TikTokOrderStatus(detail.Status),
		PaymentStatus:     detail.PaymentMethodName,
		BuyerInfo:         buyerInfo,
		ShippingAddress:   shippingAddr,
		TotalAmount:       totalAmount,
		Currency:          detail.Payment.Currency,
		PlacedAt:          placedAt,
		PaidAt:            paidAt,
		RawPayload:        rawPayload,
		SyncState:         models.SyncStateNew,
	}

	result := database.DB.Where("tik_tok_order_id = ?", detail.ID).
		Assign(order).
		FirstOrCreate(&order)

	if result.Error != nil {
		return fmt.Errorf("failed to upsert order: %w", result.Error)
	}

	for _, item := range detail.LineItems {
		price := 0.0
		if item.SalePrice != "" {
			fmt.Sscanf(item.SalePrice, "%f", &price)
		}

		orderItem := models.OrderItem{
			OrderID:           order.ID,
			TikTokOrderItemID: item.ID,
			TikTokProductID:   item.ProductID,
			TikTokSKUID:       item.SkuID,
			SellerSKU:         item.SellerSku,
			Qty:               item.Quantity,
			Price:             price,
		}

		database.DB.Where("order_id = ? AND tik_tok_order_item_id = ?", order.ID, item.ID).
			Assign(orderItem).
			FirstOrCreate(&orderItem)
	}

	logger.Info("order upserted",
		zap.String("tiktok_order_id", detail.ID),
		zap.String("status", detail.Status),
		zap.Int("items_count", len(detail.LineItems)))

	return nil
}

func processPackageEvent(ctx context.Context, event *models.WebhookEvent) error {
	logger.Info("processing package event", zap.Uint("event_id", event.ID))
	return nil
}

type InventoryPayload struct {
	ShopID     uint   `json:"shop_id"`
	ShopCipher string `json:"shop_cipher"`
	SKUID      uint   `json:"sku_id"` // Changed from LocalSimID to SKUID
}

func HandleSyncInventory(ctx context.Context, task *asynq.Task) error {
	var payload InventoryPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	logger.Info("syncing inventory for SKU", zap.Uint("sku_id", payload.SKUID))

	cfg := config.Get()
	inventoryService := services.NewInventorySyncService(cfg)

	if err := inventoryService.SyncSKUInventoryToTikTok(ctx, payload.ShopID, payload.ShopCipher, payload.SKUID); err != nil {
		return fmt.Errorf("failed to sync inventory: %w", err)
	}

	return nil
}

type ShipPayload struct {
	ShopID         uint   `json:"shop_id"`
	ShopCipher     string `json:"shop_cipher"`
	TikTokOrderID  string `json:"tiktok_order_id"`
	TrackingNumber string `json:"tracking_number"`
	CarrierID      string `json:"carrier_id"`
}

func HandleShipPackage(ctx context.Context, task *asynq.Task) error {
	var payload ShipPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	logger.Info("shipping package for order", zap.String("tiktok_order_id", payload.TikTokOrderID))

	cfg := config.Get()
	fulfillmentService := services.NewFulfillmentSyncService(cfg)

	err := fulfillmentService.ShipOrder(ctx, &services.ShipOrderRequest{
		ShopID:         payload.ShopID,
		ShopCipher:     payload.ShopCipher,
		TikTokOrderID:  payload.TikTokOrderID,
		TrackingNumber: payload.TrackingNumber,
		CarrierID:      payload.CarrierID,
	})

	if err != nil {
		return fmt.Errorf("failed to ship package: %w", err)
	}

	return nil
}

func HandleReconcileOrders(ctx context.Context, task *asynq.Task) error {
	logger.Info("running order reconciliation")

	cfg := config.Get()
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)
	ordersAPI := tiktok.NewOrdersAPI(client, tokenManager)
	orderSyncService := services.NewOrderSyncService()

	var shops []models.Shop
	if err := database.DB.Where("status = ?", "active").Find(&shops).Error; err != nil {
		return fmt.Errorf("failed to get shops: %w", err)
	}

	for _, shop := range shops {
		var token models.OAuthToken
		if err := database.DB.Where("shop_id = ?", shop.ID).First(&token).Error; err != nil {
			logger.Warn("no token for shop, skipping", zap.String("shop_id", shop.ShopID))
			continue
		}

		orders, err := ordersAPI.GetRecentOrders(ctx, shop.ID, "", 24*time.Hour)
		if err != nil {
			logger.Error("failed to get recent orders for shop",
				zap.String("shop_id", shop.ShopID),
				zap.Error(err))
			continue
		}

		logger.Info("reconciling orders for shop",
			zap.Int("order_count", len(orders)),
			zap.String("shop_id", shop.ShopID))

		for _, orderSummary := range orders {
			var existingOrder models.Order
			err := database.DB.Where("tik_tok_order_id = ?", orderSummary.ID).First(&existingOrder).Error

			if err == nil {
				if string(existingOrder.TikTokOrderStatus) != orderSummary.Status {
					existingOrder.TikTokOrderStatus = models.TikTokOrderStatus(orderSummary.Status)
					existingOrder.SyncState = models.SyncStateNew
					database.DB.Save(&existingOrder)
					logger.Info("updated order status",
						zap.String("order_id", orderSummary.ID),
						zap.String("new_status", orderSummary.Status))
				}
				continue
			}

			orderDetail, err := ordersAPI.GetOrderDetail(ctx, shop.ID, "", orderSummary.ID)
			if err != nil {
				logger.Error("failed to get order detail",
					zap.String("order_id", orderSummary.ID),
					zap.Error(err))
				continue
			}

			if err := upsertOrder(shop.ID, shop.ShopID, orderDetail, nil); err != nil {
				logger.Error("failed to upsert order",
					zap.String("order_id", orderSummary.ID),
					zap.Error(err))
				continue
			}

			var newOrder models.Order
			database.DB.Where("tik_tok_order_id = ?", orderSummary.ID).First(&newOrder)
			orderSyncService.SyncOrderToLocal(ctx, &newOrder)
		}
	}

	pendingOrders, _ := orderSyncService.GetPendingSyncOrders(ctx)
	for _, order := range pendingOrders {
		orderSyncService.SyncOrderToLocal(ctx, &order)
	}

	logger.Info("order reconciliation completed")
	return nil
}

type SyncAllInventoryPayload struct {
	ShopID     uint   `json:"shop_id"`
	ShopCipher string `json:"shop_cipher"`
	ProductID  uint   `json:"product_id"` // Sync all SKUs of a product
}

func HandleSyncAllInventory(ctx context.Context, task *asynq.Task) error {
	var payload SyncAllInventoryPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	logger.Info("syncing all inventory for product", zap.Uint("product_id", payload.ProductID))

	cfg := config.Get()
	inventoryService := services.NewInventorySyncService(cfg)

	count, err := inventoryService.SyncProductInventoryToTikTok(ctx, payload.ShopID, payload.ShopCipher, payload.ProductID)
	if err != nil {
		return fmt.Errorf("failed to sync inventory: %w", err)
	}

	logger.Info("synced SKUs for product",
		zap.Int("sku_count", count),
		zap.Uint("product_id", payload.ProductID))
	return nil
}
