package services

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

type FulfillmentSyncService struct {
	cfg            *config.Config
	client         *tiktok.Client
	tokenManager   *tiktok.TokenManager
	fulfillmentAPI *tiktok.FulfillmentAPI
	logisticsAPI   *tiktok.LogisticsAPI
}

func NewFulfillmentSyncService(cfg *config.Config) *FulfillmentSyncService {
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)

	return &FulfillmentSyncService{
		cfg:            cfg,
		client:         client,
		tokenManager:   tokenManager,
		fulfillmentAPI: tiktok.NewFulfillmentAPI(client, tokenManager),
		logisticsAPI:   tiktok.NewLogisticsAPI(client, tokenManager),
	}
}

type ShipOrderRequest struct {
	ShopID         string
	ShopCipher     string
	TikTokOrderID  string
	TrackingNumber string
	CarrierID      string
	CarrierName    string
}

func (s *FulfillmentSyncService) ShipOrder(ctx context.Context, req *ShipOrderRequest) error {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", req.ShopID).First(&shop).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	var order models.Order
	err := database.DB.Where("tik_tok_shop_id = ? AND tik_tok_order_id = ?", req.ShopID, req.TikTokOrderID).
		First(&order).Error
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if order.TikTokOrderStatus != models.OrderStatusAwaitingShipment {
		return fmt.Errorf("order status is %s, expected AWAITING_SHIPMENT", order.TikTokOrderStatus)
	}

	resp, err := s.fulfillmentAPI.ShipPackage(ctx, req.ShopID, req.ShopCipher, &tiktok.ShipPackageRequest{
		OrderID:            req.TikTokOrderID,
		TrackingNumber:     req.TrackingNumber,
		ShippingProviderID: req.CarrierID,
	})

	if err != nil {
		return fmt.Errorf("failed to ship package: %w", err)
	}

	if len(resp.FailedOrderIDs) > 0 {
		return fmt.Errorf("ship failed for orders: %v", resp.FailedOrderIDs)
	}

	order.TikTokOrderStatus = models.OrderStatusAwaitingCollection
	database.DB.Save(&order)

	logger.Info("order shipped",
		zap.String("tiktok_order_id", req.TikTokOrderID),
		zap.String("tracking_number", req.TrackingNumber),
		zap.String("carrier_id", req.CarrierID))

	return nil
}

func (s *FulfillmentSyncService) MarkDelivered(ctx context.Context, tiktokShopID string, shopCipher, packageID string) error {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	err := s.fulfillmentAPI.MarkPackageDelivered(ctx, tiktokShopID, shopCipher, &tiktok.MarkPackageDeliveredRequest{
		PackageID: packageID,
	})

	if err != nil {
		return fmt.Errorf("failed to mark delivered: %w", err)
	}

	logger.Info("package marked as delivered", zap.String("package_id", packageID))
	return nil
}

func (s *FulfillmentSyncService) GetShippingProviders(ctx context.Context, tiktokShopID string, shopCipher string) ([]tiktok.ShippingProvider, error) {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return nil, fmt.Errorf("shop not found: %w", err)
	}
	return s.logisticsAPI.GetShippingProviders(ctx, tiktokShopID, shopCipher)
}

func (s *FulfillmentSyncService) GetWarehouses(ctx context.Context, tiktokShopID string, shopCipher string) ([]tiktok.Warehouse, error) {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return nil, fmt.Errorf("shop not found: %w", err)
	}
	return s.logisticsAPI.GetWarehouses(ctx, tiktokShopID, shopCipher)
}

func (s *FulfillmentSyncService) GetOrdersReadyToShip(ctx context.Context, tiktokShopID string) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("tik_tok_shop_id = ? AND tik_tok_order_status = ?",
		tiktokShopID, models.OrderStatusAwaitingShipment).
		Order("created_at ASC").
		Find(&orders).Error

	return orders, err
}

func (s *FulfillmentSyncService) GetOrdersInTransit(ctx context.Context, tiktokShopID string) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("tik_tok_shop_id = ? AND tik_tok_order_status IN ?",
		tiktokShopID, []string{
			string(models.OrderStatusAwaitingCollection),
			string(models.OrderStatusInTransit),
		}).
		Order("created_at ASC").
		Find(&orders).Error

	return orders, err
}
