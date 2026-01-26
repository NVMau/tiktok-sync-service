package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

// =============================================================================
// ProductSyncService - Quản lý đồng bộ Products và SKUs với TikTok
// =============================================================================

type ProductSyncService struct {
	cfg          *config.Config
	client       *tiktok.Client
	tokenManager *tiktok.TokenManager
	productsAPI  *tiktok.ProductsAPI
}

func NewProductSyncService(cfg *config.Config) *ProductSyncService {
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)

	return &ProductSyncService{
		cfg:          cfg,
		client:       client,
		tokenManager: tokenManager,
		productsAPI:  tiktok.NewProductsAPI(client, tokenManager),
	}
}

// =============================================================================
// PRODUCT OPERATIONS
// =============================================================================

// SyncProductsFromTikTok - Đồng bộ tất cả products từ TikTok về DB
func (s *ProductSyncService) SyncProductsFromTikTok(ctx context.Context, shopID uint, shopCipher string) (int, error) {
	var shop models.Shop
	if err := database.DB.First(&shop, shopID).Error; err != nil {
		return 0, fmt.Errorf("shop not found: %w", err)
	}

	// Fetch products from TikTok
	resp, err := s.productsAPI.GetProductList(ctx, &tiktok.ProductListRequest{
		ShopID:     shopID,
		ShopCipher: shopCipher,
		PageSize:   100,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch products: %w", err)
	}

	syncedCount := 0
	for _, p := range resp.Products {
		// Upsert product
		product := models.Product{
			ShopID:          shopID,
			TikTokProductID: p.ID,
			Title:           p.Title,
			Status:          models.ProductStatus(p.Status),
		}

		result := database.DB.Where("tiktok_product_id = ?", p.ID).
			Assign(product).
			FirstOrCreate(&product)

		if result.Error != nil {
			log.Printf("Failed to upsert product %s: %v", p.ID, result.Error)
			continue
		}

		// Fetch and sync SKUs for this product
		if err := s.SyncSKUsFromTikTok(ctx, shopID, shopCipher, &product); err != nil {
			log.Printf("Failed to sync SKUs for product %s: %v", p.ID, err)
		}

		syncedCount++
	}

	log.Printf("Synced %d products from TikTok for shop %d", syncedCount, shopID)
	return syncedCount, nil
}

// SyncSKUsFromTikTok - Đồng bộ SKUs của 1 product từ TikTok
func (s *ProductSyncService) SyncSKUsFromTikTok(ctx context.Context, shopID uint, shopCipher string, product *models.Product) error {
	detail, err := s.productsAPI.GetProductDetail(ctx, shopID, shopCipher, product.TikTokProductID)
	if err != nil {
		return fmt.Errorf("failed to get product detail: %w", err)
	}

	// Update product info
	product.Title = detail.Title
	product.Description = detail.Description
	product.CategoryID = detail.CategoryID
	product.Status = models.ProductStatus(detail.Status)

	if detail.MainImages != nil {
		imagesJSON, _ := json.Marshal(detail.MainImages)
		product.MainImages = imagesJSON
	}

	rawJSON, _ := json.Marshal(detail)
	product.RawData = rawJSON

	now := time.Now()
	product.SyncedAt = &now
	product.SKUCount = len(detail.SKUs)
	database.DB.Save(product)

	// Sync each SKU
	for _, sku := range detail.SKUs {
		tiktokSKUID := sku.ID

		// Calculate total quantity from all warehouses
		totalQty := 0
		for _, inv := range sku.Inventory {
			totalQty += inv.Quantity
		}

		// Parse price
		var price float64
		if sku.Price.SalePrice != "" {
			fmt.Sscanf(sku.Price.SalePrice, "%f", &price)
		}

		var originalPrice *float64
		if sku.Price.OriginalPrice != "" {
			var op float64
			fmt.Sscanf(sku.Price.OriginalPrice, "%f", &op)
			originalPrice = &op
		}

		// Inventory info JSON
		invJSON, _ := json.Marshal(sku.Inventory)

		// Sales attributes JSON
		attrJSON, _ := json.Marshal(sku.SalesAttributes)

		// Upsert SKU
		skuModel := models.SKU{
			ProductID:       product.ID,
			TikTokSKUID:     &tiktokSKUID,
			SellerSKU:       sku.SellerSku,
			Price:           price,
			OriginalPrice:   originalPrice,
			Quantity:        totalQty,
			SyncStatus:      models.SKUSyncStatusSynced, // Đã có trên TikTok
			SaleStatus:      models.SKUSaleStatusAvailable,
			SalesAttributes: attrJSON,
			InventoryInfo:   invJSON,
			SyncedAt:        &now,
		}

		// Determine sale status based on quantity
		if totalQty == 0 {
			skuModel.SaleStatus = models.SKUSaleStatusSold
		}

		result := database.DB.Where("seller_sku = ?", sku.SellerSku).
			Assign(skuModel).
			FirstOrCreate(&skuModel)

		if result.Error != nil {
			log.Printf("Failed to upsert SKU %s: %v", sku.SellerSku, result.Error)
			continue
		}

		log.Printf("Synced SKU: %s (TikTok ID: %s, qty: %d)", sku.SellerSku, sku.ID, totalQty)
	}

	return nil
}

// GetProduct - Lấy thông tin product theo ID
func (s *ProductSyncService) GetProduct(ctx context.Context, productID uint) (*models.Product, error) {
	var product models.Product
	err := database.DB.Preload("SKUs").First(&product, productID).Error
	return &product, err
}

// GetProductByTikTokID - Lấy product theo TikTok Product ID
func (s *ProductSyncService) GetProductByTikTokID(ctx context.Context, tiktokProductID string) (*models.Product, error) {
	var product models.Product
	err := database.DB.Preload("SKUs").
		Where("tiktok_product_id = ?", tiktokProductID).
		First(&product).Error
	return &product, err
}

// ListProducts - Danh sách products của shop
func (s *ProductSyncService) ListProducts(ctx context.Context, shopID uint, status string) ([]models.Product, error) {
	var products []models.Product
	query := database.DB.Where("shop_id = ?", shopID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Preload("SKUs").Order("created_at DESC").Find(&products).Error
	return products, err
}

// =============================================================================
// SKU OPERATIONS
// =============================================================================

// CreateSKU - Tạo SKU mới (từ external system), chờ push lên TikTok
func (s *ProductSyncService) CreateSKU(ctx context.Context, productID uint, sellerSKU string, price float64) (*models.SKU, error) {
	// Check product exists
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// Check duplicate seller_sku
	var existing models.SKU
	if err := database.DB.Where("seller_sku = ?", sellerSKU).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("seller_sku %s already exists", sellerSKU)
	}

	sku := models.SKU{
		ProductID:  productID,
		SellerSKU:  sellerSKU,
		Price:      price,
		Quantity:   1,
		SyncStatus: models.SKUSyncStatusPending, // Chờ push lên TikTok
		SaleStatus: models.SKUSaleStatusAvailable,
	}

	if err := database.DB.Create(&sku).Error; err != nil {
		return nil, err
	}

	// Update product sku_count
	database.DB.Model(&product).UpdateColumn("sku_count", product.SKUCount+1)

	log.Printf("Created new SKU: %s (pending push to TikTok)", sellerSKU)
	return &sku, nil
}

// CreateSKUsBatch - Tạo nhiều SKUs cùng lúc
func (s *ProductSyncService) CreateSKUsBatch(ctx context.Context, productID uint, skuRequests []SKUCreateRequest) ([]models.SKU, []error) {
	var createdSKUs []models.SKU
	var errors []error

	for _, req := range skuRequests {
		sku, err := s.CreateSKU(ctx, productID, req.SellerSKU, req.Price)
		if err != nil {
			errors = append(errors, fmt.Errorf("SKU %s: %w", req.SellerSKU, err))
			continue
		}
		createdSKUs = append(createdSKUs, *sku)
	}

	return createdSKUs, errors
}

type SKUCreateRequest struct {
	SellerSKU string  `json:"seller_sku"` // Số điện thoại
	Price     float64 `json:"price"`
}

// GetSKU - Lấy SKU theo ID
func (s *ProductSyncService) GetSKU(ctx context.Context, skuID uint) (*models.SKU, error) {
	var sku models.SKU
	err := database.DB.Preload("Product").First(&sku, skuID).Error
	return &sku, err
}

// GetSKUBySellerSKU - Lấy SKU theo số điện thoại
func (s *ProductSyncService) GetSKUBySellerSKU(ctx context.Context, sellerSKU string) (*models.SKU, error) {
	var sku models.SKU
	err := database.DB.Preload("Product").
		Where("seller_sku = ?", sellerSKU).
		First(&sku).Error
	return &sku, err
}

// ListSKUs - Danh sách SKUs của product
func (s *ProductSyncService) ListSKUs(ctx context.Context, productID uint, syncStatus, saleStatus string) ([]models.SKU, error) {
	var skus []models.SKU
	query := database.DB.Where("product_id = ?", productID)

	if syncStatus != "" {
		query = query.Where("sync_status = ?", syncStatus)
	}
	if saleStatus != "" {
		query = query.Where("sale_status = ?", saleStatus)
	}

	err := query.Order("created_at DESC").Find(&skus).Error
	return skus, err
}

// GetPendingSKUs - Lấy các SKU đang chờ push lên TikTok
func (s *ProductSyncService) GetPendingSKUs(ctx context.Context, shopID uint) ([]models.SKU, error) {
	var skus []models.SKU
	err := database.DB.Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
		Where("products.shop_id = ?", shopID).
		Where("skus.sync_status IN ?", []string{
			string(models.SKUSyncStatusPending),
			string(models.SKUSyncStatusFailed),
		}).
		Find(&skus).Error
	return skus, err
}

// UpdateSKUPrice - Cập nhật giá SKU
func (s *ProductSyncService) UpdateSKUPrice(ctx context.Context, skuID uint, price float64) error {
	return database.DB.Model(&models.SKU{}).
		Where("id = ?", skuID).
		Updates(map[string]interface{}{
			"price":      price,
			"updated_at": time.Now(),
		}).Error
}

// ReserveSKU - Đặt trước SKU khi có đơn hàng
func (s *ProductSyncService) ReserveSKU(ctx context.Context, skuID uint) error {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return err
	}

	if !sku.IsSellable() {
		return fmt.Errorf("SKU %s is not available for sale", sku.SellerSKU)
	}

	sku.Reserve()
	return database.DB.Save(&sku).Error
}

// MarkSKUAsSold - Đánh dấu SKU đã bán
func (s *ProductSyncService) MarkSKUAsSold(ctx context.Context, skuID uint) error {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return err
	}

	sku.MarkAsSold()
	return database.DB.Save(&sku).Error
}

// ReleaseSKU - Giải phóng SKU khi đơn bị hủy
func (s *ProductSyncService) ReleaseSKU(ctx context.Context, skuID uint) error {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return err
	}

	sku.Release()
	return database.DB.Save(&sku).Error
}

// =============================================================================
// TIKTOK SYNC OPERATIONS
// =============================================================================

// DeactivateProduct - Vô hiệu hóa product trên TikTok
func (s *ProductSyncService) DeactivateProduct(ctx context.Context, shopID uint, shopCipher string, productID uint) error {
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	err := s.productsAPI.DeactivateProducts(ctx, shopID, shopCipher, []string{product.TikTokProductID})
	if err != nil {
		return fmt.Errorf("failed to deactivate on TikTok: %w", err)
	}

	product.Status = models.ProductStatusInactive
	database.DB.Save(&product)

	log.Printf("Deactivated product: %s", product.TikTokProductID)
	return nil
}

// ActivateProduct - Kích hoạt product trên TikTok
func (s *ProductSyncService) ActivateProduct(ctx context.Context, shopID uint, shopCipher string, productID uint) error {
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	err := s.productsAPI.ActivateProducts(ctx, shopID, shopCipher, []string{product.TikTokProductID})
	if err != nil {
		return fmt.Errorf("failed to activate on TikTok: %w", err)
	}

	product.Status = models.ProductStatusActive
	database.DB.Save(&product)

	log.Printf("Activated product: %s", product.TikTokProductID)
	return nil
}

// UpdatePriceOnTikTok - Cập nhật giá SKU trên TikTok
func (s *ProductSyncService) UpdatePriceOnTikTok(ctx context.Context, shopID uint, shopCipher string, skuID uint, price float64) error {
	var sku models.SKU
	if err := database.DB.Preload("Product").First(&sku, skuID).Error; err != nil {
		return fmt.Errorf("SKU not found: %w", err)
	}

	if sku.TikTokSKUID == nil {
		return fmt.Errorf("SKU %s not pushed to TikTok yet", sku.SellerSKU)
	}

	priceStr := fmt.Sprintf("%.2f", price)
	err := s.productsAPI.UpdatePrice(ctx, shopID, shopCipher, &tiktok.UpdatePriceRequest{
		ProductID: sku.Product.TikTokProductID,
		SKUs: []tiktok.UpdateSKUPrice{
			{
				ID:        *sku.TikTokSKUID,
				SalePrice: priceStr,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to update price on TikTok: %w", err)
	}

	sku.Price = price
	now := time.Now()
	sku.SyncedAt = &now
	database.DB.Save(&sku)

	log.Printf("Updated price on TikTok: SKU %s, price=%.2f", sku.SellerSKU, price)
	return nil
}
