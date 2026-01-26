package services

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

// =============================================================================
// InventorySyncService - Quản lý đồng bộ tồn kho với TikTok
// =============================================================================
// Trong business SIM số đẹp:
//   - Mỗi SKU có quantity = 0 hoặc 1 (vì số điện thoại là unique)
//   - Khi bán: quantity 1 → 0
//   - Không có concept "restock" như hàng hóa thông thường

type InventorySyncService struct {
	cfg          *config.Config
	client       *tiktok.Client
	tokenManager *tiktok.TokenManager
	productsAPI  *tiktok.ProductsAPI
}

func NewInventorySyncService(cfg *config.Config) *InventorySyncService {
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)

	return &InventorySyncService{
		cfg:          cfg,
		client:       client,
		tokenManager: tokenManager,
		productsAPI:  tiktok.NewProductsAPI(client, tokenManager),
	}
}

// =============================================================================
// SYNC TO TIKTOK
// =============================================================================

// SyncSKUInventoryToTikTok - Đồng bộ tồn kho của 1 SKU lên TikTok
func (s *InventorySyncService) SyncSKUInventoryToTikTok(ctx context.Context, shopID uint, shopCipher string, skuID uint) error {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return fmt.Errorf("SKU not found: %w", err)
	}

	if sku.TikTokSKUID == nil {
		return fmt.Errorf("SKU %s not pushed to TikTok yet", sku.SellerSKU)
	}

	// Load product separately (gorm:"-" prevents Preload)
	var product models.Product
	if err := database.DB.First(&product, sku.ProductID).Error; err != nil {
		return fmt.Errorf("product not found for SKU %s: %w", sku.SellerSKU, err)
	}

	// Call TikTok API to update inventory
	err := s.productsAPI.UpdateInventory(ctx, shopID, shopCipher, &tiktok.UpdateInventoryRequest{
		ProductID: product.TikTokProductID,
		SKUs: []tiktok.UpdateSKUInventory{
			{
				ID: *sku.TikTokSKUID,
				Inventory: []tiktok.InventoryUpdate{
					{
						WarehouseID: "default",
						Quantity:    sku.Quantity,
					},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to update TikTok inventory: %w", err)
	}

	// Update sync time
	now := time.Now()
	sku.SyncedAt = &now
	database.DB.Save(&sku)

	logger.Info("synced inventory to TikTok",
		zap.String("seller_sku", sku.SellerSKU),
		zap.Int("quantity", sku.Quantity))
	return nil
}

// SyncProductInventoryToTikTok - Đồng bộ tồn kho tất cả SKUs của 1 product
func (s *InventorySyncService) SyncProductInventoryToTikTok(ctx context.Context, shopID uint, shopCipher string, productID uint) (int, error) {
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return 0, fmt.Errorf("product not found: %w", err)
	}

	// Load SKUs separately (gorm:"-" prevents Preload)
	var skus []models.SKU
	if err := database.DB.Where("product_id = ?", productID).Find(&skus).Error; err != nil {
		return 0, fmt.Errorf("failed to load SKUs: %w", err)
	}

	// Build batch update request
	var skuUpdates []tiktok.UpdateSKUInventory
	for _, sku := range skus {
		if sku.TikTokSKUID == nil {
			continue // Skip SKUs not pushed yet
		}
		skuUpdates = append(skuUpdates, tiktok.UpdateSKUInventory{
			ID: *sku.TikTokSKUID,
			Inventory: []tiktok.InventoryUpdate{
				{
					WarehouseID: "default",
					Quantity:    sku.Quantity,
				},
			},
		})
	}

	if len(skuUpdates) == 0 {
		logger.Debug("no SKUs to sync for product", zap.Uint("product_id", productID))
		return 0, nil
	}

	err := s.productsAPI.UpdateInventory(ctx, shopID, shopCipher, &tiktok.UpdateInventoryRequest{
		ProductID: product.TikTokProductID,
		SKUs:      skuUpdates,
	})

	if err != nil {
		return 0, fmt.Errorf("failed to update TikTok inventory: %w", err)
	}

	// Update sync time for all SKUs
	now := time.Now()
	for _, sku := range product.SKUs {
		if sku.TikTokSKUID != nil {
			sku.SyncedAt = &now
			database.DB.Save(&sku)
		}
	}

	logger.Info("synced inventory for product",
		zap.Uint("product_id", productID),
		zap.Int("sku_count", len(skuUpdates)))
	return len(skuUpdates), nil
}

// =============================================================================
// INVENTORY STATE CHANGES
// =============================================================================

// MarkSKUAsSold - Đánh dấu SKU đã bán (quantity = 0)
// Gọi khi đơn hàng được COMPLETED
func (s *InventorySyncService) MarkSKUAsSold(ctx context.Context, skuID uint) error {
	return database.DB.Model(&models.SKU{}).
		Where("id = ?", skuID).
		Updates(map[string]interface{}{
			"quantity":    0,
			"sale_status": models.SKUSaleStatusSold,
			"updated_at":  time.Now(),
		}).Error
}

// ReserveSKU - Đặt trước SKU khi có đơn hàng mới
// Tránh bán trùng số (double selling)
func (s *InventorySyncService) ReserveSKU(ctx context.Context, skuID uint) error {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return err
	}

	if sku.SaleStatus != models.SKUSaleStatusAvailable {
		return fmt.Errorf("SKU %s is not available (status: %s)", sku.SellerSKU, sku.SaleStatus)
	}

	return database.DB.Model(&sku).
		Updates(map[string]interface{}{
			"sale_status": models.SKUSaleStatusReserved,
			"updated_at":  time.Now(),
		}).Error
}

// ReleaseSKU - Giải phóng SKU khi đơn bị hủy
func (s *InventorySyncService) ReleaseSKU(ctx context.Context, skuID uint) error {
	return database.DB.Model(&models.SKU{}).
		Where("id = ?", skuID).
		Updates(map[string]interface{}{
			"quantity":    1,
			"sale_status": models.SKUSaleStatusAvailable,
			"updated_at":  time.Now(),
		}).Error
}

// =============================================================================
// STATISTICS
// =============================================================================

// InventoryStats - Thống kê tồn kho
type InventoryStats struct {
	TotalSKUs     int64 `json:"total_skus"`
	AvailableSKUs int64 `json:"available_skus"`
	ReservedSKUs  int64 `json:"reserved_skus"`
	SoldSKUs      int64 `json:"sold_skus"`
	PendingPush   int64 `json:"pending_push"`  // SKU chờ push lên TikTok
	FailedPush    int64 `json:"failed_push"`   // SKU push thất bại
}

// GetInventoryStats - Lấy thống kê tồn kho của shop
func (s *InventorySyncService) GetInventoryStats(ctx context.Context, shopID uint) (*InventoryStats, error) {
	stats := &InventoryStats{}

	// Total SKUs
	database.DB.Model(&models.SKU{}).
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Count(&stats.TotalSKUs)

	// Available
	database.DB.Model(&models.SKU{}).
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sale_status = ?", models.SKUSaleStatusAvailable).
		Count(&stats.AvailableSKUs)

	// Reserved
	database.DB.Model(&models.SKU{}).
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sale_status = ?", models.SKUSaleStatusReserved).
		Count(&stats.ReservedSKUs)

	// Sold
	database.DB.Model(&models.SKU{}).
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sale_status = ?", models.SKUSaleStatusSold).
		Count(&stats.SoldSKUs)

	// Pending push
	database.DB.Model(&models.SKU{}).
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sync_status = ?", models.SKUSyncStatusPending).
		Count(&stats.PendingPush)

	// Failed push
	database.DB.Model(&models.SKU{}).
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sync_status = ?", models.SKUSyncStatusFailed).
		Count(&stats.FailedPush)

	return stats, nil
}

// GetSKUsByStatus - Lấy danh sách SKUs theo trạng thái
func (s *InventorySyncService) GetSKUsByStatus(ctx context.Context, shopID uint, saleStatus models.SKUSaleStatus, limit int) ([]models.SKU, error) {
	var skus []models.SKU

	query := database.DB.
		Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sale_status = ?", saleStatus).
		Order("skus.updated_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&skus).Error; err != nil {
		return nil, err
	}

	// Load products separately if needed (gorm:"-" prevents Preload)
	// For now, we return SKUs without preloaded Product
	return skus, nil
}
