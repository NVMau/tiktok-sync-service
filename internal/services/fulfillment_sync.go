package services

import (
	"context"
	"fmt"
	"log"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
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
	ShopID         uint
	ShopCipher     string
	TikTokOrderID  string
	TrackingNumber string
	CarrierID      string
	CarrierName    string
}

func (s *FulfillmentSyncService) ShipOrder(ctx context.Context, req *ShipOrderRequest) error {
	var order models.Order
	err := database.DB.Where("shop_id = ? AND tik_tok_order_id = ?", req.ShopID, req.TikTokOrderID).
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

	log.Printf("Order shipped: %s, tracking=%s, carrier=%s",
		req.TikTokOrderID, req.TrackingNumber, req.CarrierID)

	return nil
}

func (s *FulfillmentSyncService) MarkDelivered(ctx context.Context, shopID uint, shopCipher, packageID string) error {
	err := s.fulfillmentAPI.MarkPackageDelivered(ctx, shopID, shopCipher, &tiktok.MarkPackageDeliveredRequest{
		PackageID: packageID,
	})

	if err != nil {
		return fmt.Errorf("failed to mark delivered: %w", err)
	}

	log.Printf("Package marked as delivered: %s", packageID)
	return nil
}

func (s *FulfillmentSyncService) GetShippingProviders(ctx context.Context, shopID uint, shopCipher string) ([]tiktok.ShippingProvider, error) {
	return s.logisticsAPI.GetShippingProviders(ctx, shopID, shopCipher)
}

func (s *FulfillmentSyncService) GetWarehouses(ctx context.Context, shopID uint, shopCipher string) ([]tiktok.Warehouse, error) {
	return s.logisticsAPI.GetWarehouses(ctx, shopID, shopCipher)
}

func (s *FulfillmentSyncService) GetOrdersReadyToShip(ctx context.Context, shopID uint) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("shop_id = ? AND tik_tok_order_status = ?",
		shopID, models.OrderStatusAwaitingShipment).
		Order("created_at ASC").
		Find(&orders).Error

	return orders, err
}

func (s *FulfillmentSyncService) GetOrdersInTransit(ctx context.Context, shopID uint) ([]models.Order, error) {
	var orders []models.Order
	err := database.DB.Where("shop_id = ? AND tik_tok_order_status IN ?",
		shopID, []string{
			string(models.OrderStatusAwaitingCollection),
			string(models.OrderStatusInTransit),
		}).
		Order("created_at ASC").
		Find(&orders).Error

	return orders, err
}
