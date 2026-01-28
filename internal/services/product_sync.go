package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

// ProductSyncService handles synchronization of products and SKUs with TikTok
type ProductSyncService struct {
	cfg          *config.Config
	client       *tiktok.Client
	tokenManager *tiktok.TokenManager
	productsAPI  *tiktok.ProductsAPI
	log          *zap.Logger
}

// NewProductSyncService creates a new ProductSyncService instance
func NewProductSyncService(cfg *config.Config) *ProductSyncService {
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)

	return &ProductSyncService{
		cfg:          cfg,
		client:       client,
		tokenManager: tokenManager,
		productsAPI:  tiktok.NewProductsAPI(client, tokenManager),
		log:          logger.Log.Named("product_sync"),
	}
}

// =============================================================================
// PRODUCT SYNC OPERATIONS
// =============================================================================

// SyncProductsFromTikTok fetches and syncs all products from TikTok to local DB
// Uses status="ALL" to include products with all statuses (ACTIVATE, SELLER_DEACTIVATED, DELETED, etc.)
// to ensure SKUs from historical orders can be looked up
func (s *ProductSyncService) SyncProductsFromTikTok(ctx context.Context, tiktokShopID string, shopCipher string) (int, error) {
	log := s.log.With(zap.String("tiktok_shop_id", tiktokShopID))

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		log.Error("shop not found", zap.Error(err))
		return 0, fmt.Errorf("shop not found: %w", err)
	}

	syncedCount := 0
	pageToken := ""

	for {
		// Use status="ALL" to get products with all statuses including DELETED
		resp, err := s.productsAPI.GetProductList(ctx, &tiktok.ProductListRequest{
			TikTokShopID: tiktokShopID,
			ShopCipher:   shopCipher,
			PageSize:     100,
			PageToken:    pageToken,
			Status:       "ALL",
		})
		if err != nil {
			log.Error("failed to fetch products from TikTok", zap.Error(err))
			return syncedCount, fmt.Errorf("failed to fetch products: %w", err)
		}

		log.Info("fetched products page",
			zap.Int("count", len(resp.Products)),
			zap.Int("total", resp.TotalCount))

		for _, p := range resp.Products {
			product := models.Product{
				TikTokShopID:    shop.ShopID,
				TikTokProductID: p.ID,
				Title:           p.Title,
				Status:          models.ProductStatus(p.Status),
			}

			result := database.DB.Where("tik_tok_product_id = ?", p.ID).
				Assign(product).
				FirstOrCreate(&product)

			if result.Error != nil {
				log.Warn("failed to upsert product",
					zap.String("tiktok_product_id", p.ID),
					zap.Error(result.Error))
				continue
			}

			if err := s.SyncSKUsFromTikTok(ctx, tiktokShopID, shopCipher, &product); err != nil {
				log.Warn("failed to sync SKUs for product",
					zap.String("tiktok_product_id", p.ID),
					zap.String("status", p.Status),
					zap.Error(err))
			}

			syncedCount++
		}

		// Check for next page
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	log.Info("products synced from TikTok",
		zap.Int("synced_count", syncedCount))

	return syncedCount, nil
}

// SyncSKUsFromTikTok syncs SKUs of a product from TikTok
func (s *ProductSyncService) SyncSKUsFromTikTok(ctx context.Context, tiktokShopID string, shopCipher string, product *models.Product) error {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	detail, err := s.productsAPI.GetProductDetail(ctx, tiktokShopID, shopCipher, product.TikTokProductID)
	if err != nil {
		return fmt.Errorf("failed to get product detail: %w", err)
	}

	s.updateProductFromDetail(product, detail)

	for _, sku := range detail.SKUs {
		if err := s.upsertSKUFromTikTok(product.TikTokProductID, &sku); err != nil {
			s.log.Warn("failed to upsert SKU",
				zap.String("seller_sku", sku.SellerSku),
				zap.Error(err))
		}
	}

	return nil
}

// updateProductFromDetail updates product fields from TikTok detail response
func (s *ProductSyncService) updateProductFromDetail(product *models.Product, detail *tiktok.ProductDetail) {
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
}

// upsertSKUFromTikTok creates or updates a SKU from TikTok data
// Uses tik_tok_sku_id as primary lookup key (unique from TikTok)
func (s *ProductSyncService) upsertSKUFromTikTok(tiktokProductID string, tikSKU *tiktok.ProductSKU) error {
	totalQty := 0
	for _, inv := range tikSKU.Inventory {
		totalQty += inv.Quantity
	}

	var price float64
	if tikSKU.Price.SalePrice != "" {
		fmt.Sscanf(tikSKU.Price.SalePrice, "%f", &price)
	}

	var originalPrice *float64
	if tikSKU.Price.OriginalPrice != "" {
		var op float64
		fmt.Sscanf(tikSKU.Price.OriginalPrice, "%f", &op)
		originalPrice = &op
	}

	invJSON, _ := json.Marshal(tikSKU.Inventory)
	attrJSON, _ := json.Marshal(tikSKU.SalesAttributes)

	now := time.Now()
	tiktokSKUID := tikSKU.ID

	// Determine seller_sku with fallback priority:
	// 1. seller_sku from TikTok (if not empty)
	// 2. Concatenate all sales_attributes value_name (e.g., "Size-Color" or "0912345678-Red")
	// 3. tik_tok_sku_id as last resort
	sellerSKU := tikSKU.SellerSku
	if sellerSKU == "" && len(tikSKU.SalesAttributes) > 0 {
		// Combine all attribute values with "-" separator
		var attrValues []string
		for _, attr := range tikSKU.SalesAttributes {
			if attr.ValueName != "" {
				attrValues = append(attrValues, attr.ValueName)
			}
		}
		if len(attrValues) > 0 {
			sellerSKU = strings.Join(attrValues, "-")
		}
	}
	if sellerSKU == "" {
		sellerSKU = tikSKU.ID
	}

	skuModel := models.SKU{
		TikTokProductID: tiktokProductID,
		TikTokSKUID:     &tiktokSKUID,
		SellerSKU:       sellerSKU,
		Price:           price,
		OriginalPrice:   originalPrice,
		Quantity:        totalQty,
		SyncStatus:      models.SKUSyncStatusSynced,
		SaleStatus:      models.SKUSaleStatusAvailable,
		SalesAttributes: attrJSON,
		InventoryInfo:   invJSON,
		SyncedAt:        &now,
	}

	if totalQty == 0 {
		skuModel.SaleStatus = models.SKUSaleStatusSold
	}

	// Lookup by tik_tok_sku_id (unique from TikTok) instead of seller_sku
	result := database.DB.Where("tik_tok_sku_id = ?", tikSKU.ID).
		Assign(skuModel).
		FirstOrCreate(&skuModel)

	if result.Error != nil {
		return result.Error
	}

	s.log.Debug("synced SKU from TikTok",
		zap.String("seller_sku", sellerSKU),
		zap.String("tiktok_sku_id", tikSKU.ID),
		zap.Int("quantity", totalQty))

	return nil
}

// =============================================================================
// PRODUCT QUERIES
// =============================================================================

// GetProduct retrieves a product by ID with its SKUs
func (s *ProductSyncService) GetProduct(ctx context.Context, productID uint) (*models.Product, error) {
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return nil, err
	}

	var skus []models.SKU
	database.DB.Where("tik_tok_product_id = ?", product.TikTokProductID).Find(&skus)
	product.SKUs = skus

	return &product, nil
}

// GetProductByTikTokID retrieves a product by TikTok Product ID
func (s *ProductSyncService) GetProductByTikTokID(ctx context.Context, tiktokProductID string) (*models.Product, error) {
	var product models.Product
	err := database.DB.Where("tik_tok_product_id = ?", tiktokProductID).First(&product).Error
	if err != nil {
		return nil, err
	}

	var skus []models.SKU
	database.DB.Where("tik_tok_product_id = ?", tiktokProductID).Find(&skus)
	product.SKUs = skus

	return &product, nil
}

// ListProducts returns products for a shop with optional status filter
func (s *ProductSyncService) ListProducts(ctx context.Context, tiktokShopID string, status string) ([]models.Product, error) {
	var products []models.Product
	query := database.DB.Where("tik_tok_shop_id = ?", tiktokShopID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&products).Error; err != nil {
		return nil, err
	}

	for i := range products {
		var skus []models.SKU
		database.DB.Where("tik_tok_product_id = ?", products[i].TikTokProductID).Find(&skus)
		products[i].SKUs = skus
	}

	return products, nil
}

// =============================================================================
// SKU OPERATIONS
// =============================================================================

// SKUCreateRequest represents a request to create a new SKU
type SKUCreateRequest struct {
	SellerSKU string  `json:"seller_sku"`
	Price     float64 `json:"price"`
}

// CreateSKU creates a new SKU pending push to TikTok
func (s *ProductSyncService) CreateSKU(ctx context.Context, productID uint, sellerSKU string, price float64) (*models.SKU, error) {
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	var existing models.SKU
	if err := database.DB.Where("seller_sku = ?", sellerSKU).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("seller_sku %s already exists", sellerSKU)
	}

	sku := models.SKU{
		TikTokProductID: product.TikTokProductID,
		SellerSKU:       sellerSKU,
		Price:           price,
		Quantity:        1, // Phone numbers are unique
		SyncStatus:      models.SKUSyncStatusPending,
		SaleStatus:      models.SKUSaleStatusAvailable,
	}

	if err := database.DB.Create(&sku).Error; err != nil {
		return nil, fmt.Errorf("failed to create SKU: %w", err)
	}

	database.DB.Model(&product).Update("sku_count", product.SKUCount+1)

	s.log.Info("created new SKU",
		zap.Uint("product_id", productID),
		zap.String("seller_sku", sellerSKU),
		zap.Float64("price", price))

	return &sku, nil
}

// CreateSKUsBatch creates multiple SKUs at once
func (s *ProductSyncService) CreateSKUsBatch(ctx context.Context, productID uint, requests []SKUCreateRequest) ([]*models.SKU, []error) {
	var created []*models.SKU
	var errors []error

	for _, req := range requests {
		sku, err := s.CreateSKU(ctx, productID, req.SellerSKU, req.Price)
		if err != nil {
			errors = append(errors, fmt.Errorf("SKU %s: %w", req.SellerSKU, err))
			continue
		}
		created = append(created, sku)
	}

	return created, errors
}

// GetSKU retrieves a SKU by ID
func (s *ProductSyncService) GetSKU(ctx context.Context, skuID uint) (*models.SKU, error) {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return nil, err
	}
	return &sku, nil
}

// ListSKUs returns SKUs for a product with optional filters
func (s *ProductSyncService) ListSKUs(ctx context.Context, tiktokProductID string, syncStatus, saleStatus string) ([]models.SKU, error) {
	var skus []models.SKU
	query := database.DB.Where("tik_tok_product_id = ?", tiktokProductID)

	if syncStatus != "" {
		query = query.Where("sync_status = ?", syncStatus)
	}
	if saleStatus != "" {
		query = query.Where("sale_status = ?", saleStatus)
	}

	if err := query.Order("created_at DESC").Find(&skus).Error; err != nil {
		return nil, err
	}

	return skus, nil
}

// GetPendingSKUs returns all SKUs pending push for a shop
func (s *ProductSyncService) GetPendingSKUs(ctx context.Context, tiktokShopID string) ([]models.SKU, error) {
	var skus []models.SKU
	err := database.DB.
		Joins("JOIN tiktok_sync.products ON products.tik_tok_product_id = skus.tik_tok_product_id").
		Where("products.tik_tok_shop_id = ? AND skus.tik_tok_sku_id IS NULL", tiktokShopID).
		Find(&skus).Error
	return skus, err
}

// UpdateSKUPrice updates the price of a SKU locally
func (s *ProductSyncService) UpdateSKUPrice(ctx context.Context, skuID uint, price float64) error {
	return database.DB.Model(&models.SKU{}).Where("id = ?", skuID).Update("price", price).Error
}

// =============================================================================
// TIKTOK PRODUCT ACTIONS
// =============================================================================

// DeactivateProduct deactivates a product on TikTok
func (s *ProductSyncService) DeactivateProduct(ctx context.Context, tiktokShopID string, shopCipher string, productID uint) error {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	err := s.productsAPI.DeactivateProducts(ctx, tiktokShopID, shopCipher, []string{product.TikTokProductID})
	if err != nil {
		return fmt.Errorf("failed to deactivate on TikTok: %w", err)
	}

	product.Status = models.ProductStatusInactive
	database.DB.Save(&product)

	s.log.Info("deactivated product",
		zap.Uint("product_id", productID),
		zap.String("tiktok_product_id", product.TikTokProductID))

	return nil
}

// ActivateProduct activates a product on TikTok
func (s *ProductSyncService) ActivateProduct(ctx context.Context, tiktokShopID string, shopCipher string, productID uint) error {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	err := s.productsAPI.ActivateProducts(ctx, tiktokShopID, shopCipher, []string{product.TikTokProductID})
	if err != nil {
		return fmt.Errorf("failed to activate on TikTok: %w", err)
	}

	product.Status = models.ProductStatusActive
	database.DB.Save(&product)

	s.log.Info("activated product",
		zap.Uint("product_id", productID),
		zap.String("tiktok_product_id", product.TikTokProductID))

	return nil
}

// UpdatePriceOnTikTok updates SKU price on TikTok
func (s *ProductSyncService) UpdatePriceOnTikTok(ctx context.Context, tiktokShopID string, shopCipher string, skuID uint, price float64) error {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return fmt.Errorf("SKU not found: %w", err)
	}

	if sku.TikTokSKUID == nil {
		return fmt.Errorf("SKU %s not pushed to TikTok yet", sku.SellerSKU)
	}

	// Load product by TikTokProductID (gorm:"-" prevents Preload)
	var product models.Product
	if err := database.DB.Where("tik_tok_product_id = ?", sku.TikTokProductID).First(&product).Error; err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	priceStr := fmt.Sprintf("%.2f", price)
	err := s.productsAPI.UpdatePrice(ctx, tiktokShopID, shopCipher, &tiktok.UpdatePriceRequest{
		ProductID: product.TikTokProductID,
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

	s.log.Info("updated price on TikTok",
		zap.Uint("sku_id", skuID),
		zap.String("seller_sku", sku.SellerSKU),
		zap.Float64("price", price))

	return nil
}

// =============================================================================
// PUSH SKU TO TIKTOK
// =============================================================================

// PushPendingSKUsToTikTok pushes all pending SKUs of a product to TikTok
// Uses Partial Edit API - must include ALL existing SKUs to avoid deletion
func (s *ProductSyncService) PushPendingSKUsToTikTok(ctx context.Context, tiktokShopID string, shopCipher string, productID uint) (int, error) {
	log := s.log.With(
		zap.String("tiktok_shop_id", tiktokShopID),
		zap.Uint("product_id", productID))

	// Step 0: Load shop to get TikTok shop ID
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", tiktokShopID).First(&shop).Error; err != nil {
		return 0, fmt.Errorf("shop not found: %w", err)
	}

	// Step 1: Load product
	var product models.Product
	if err := database.DB.First(&product, productID).Error; err != nil {
		return 0, fmt.Errorf("product not found: %w", err)
	}

	// Step 2: Fetch current SKUs from TikTok (source of truth)
	detail, err := s.productsAPI.GetProductDetail(ctx, tiktokShopID, shopCipher, product.TikTokProductID)
	if err != nil {
		log.Error("failed to get product detail from TikTok", zap.Error(err))
		return 0, fmt.Errorf("failed to get product detail from TikTok: %w", err)
	}

	warehouseID := s.getWarehouseID(detail)
	if warehouseID == "" {
		return 0, fmt.Errorf("no warehouse found for product %s", product.TikTokProductID)
	}

	// Step 3: Get pending SKUs from DB
	var pendingSKUs []models.SKU
	if err := database.DB.Where("tik_tok_product_id = ? AND tik_tok_sku_id IS NULL", product.TikTokProductID).Find(&pendingSKUs).Error; err != nil {
		return 0, fmt.Errorf("failed to get pending SKUs: %w", err)
	}

	if len(pendingSKUs) == 0 {
		log.Debug("no pending SKUs to push")
		return 0, nil
	}

	// Step 4: Build request with existing + new SKUs
	editSKUs, pendingCount := s.buildPartialEditRequest(detail, pendingSKUs, warehouseID)

	if pendingCount == 0 {
		log.Debug("no new SKUs to push (all already exist on TikTok)")
		return 0, nil
	}

	log.Info("pushing SKUs to TikTok",
		zap.Int("new_skus", pendingCount),
		zap.Int("existing_skus", len(detail.SKUs)))

	// Step 5: Call TikTok API
	resp, err := s.productsAPI.PartialEditProduct(ctx, tiktokShopID, shopCipher, product.TikTokProductID, &tiktok.PartialEditProductRequest{
		SKUs: editSKUs,
	})
	if err != nil {
		s.markSKUsAsFailed(pendingSKUs)
		log.Error("failed to push SKUs to TikTok", zap.Error(err))
		return 0, fmt.Errorf("failed to push SKUs to TikTok: %w", err)
	}

	// Step 6: Update DB with TikTok SKU IDs
	pushedCount := s.updatePushedSKUs(product.TikTokProductID, resp.SKUs)

	log.Info("SKUs pushed successfully",
		zap.Int("pushed_count", pushedCount),
		zap.String("tiktok_product_id", product.TikTokProductID))

	return pushedCount, nil
}

// getWarehouseID extracts warehouse ID from existing SKUs
func (s *ProductSyncService) getWarehouseID(detail *tiktok.ProductDetail) string {
	for _, tikSKU := range detail.SKUs {
		if len(tikSKU.Inventory) > 0 {
			return tikSKU.Inventory[0].WarehouseID
		}
	}
	return ""
}

// buildPartialEditRequest builds the request with existing + new SKUs
func (s *ProductSyncService) buildPartialEditRequest(detail *tiktok.ProductDetail, pendingSKUs []models.SKU, warehouseID string) ([]tiktok.PartialEditSKU, int) {
	var editSKUs []tiktok.PartialEditSKU

	// Add existing SKUs (keep them, don't overwrite inventory)
	existingSellerSKUs := make(map[string]bool)
	for _, tikSKU := range detail.SKUs {
		existingSellerSKUs[tikSKU.SellerSku] = true

		price := tikSKU.Price.SalePrice
		if price == "" {
			price = tikSKU.Price.Amount
		}

		var salesAttrs []tiktok.PartialEditAttribute
		for _, attr := range tikSKU.SalesAttributes {
			salesAttrs = append(salesAttrs, tiktok.PartialEditAttribute{
				ID:        attr.ID,
				Name:      attr.Name,
				ValueID:   attr.ValueID,
				ValueName: attr.ValueName,
			})
		}

		// Keep ID + Price, skip Inventory to avoid race condition
		editSKUs = append(editSKUs, tiktok.PartialEditSKU{
			ID:              tikSKU.ID,
			SellerSKU:       tikSKU.SellerSku,
			SalesAttributes: salesAttrs,
			Price: tiktok.PartialEditPrice{
				Currency:  "VND",
				SalePrice: price,
			},
		})
	}

	// Add new pending SKUs
	pendingCount := 0
	variantAttr := s.getVariantAttribute(detail)

	for _, sku := range pendingSKUs {
		if existingSellerSKUs[sku.SellerSKU] {
			s.log.Debug("SKU already exists on TikTok, skipping",
				zap.String("seller_sku", sku.SellerSKU))
			continue
		}

		if variantAttr == nil {
			s.log.Warn("product has no variant, cannot add new SKU",
				zap.String("seller_sku", sku.SellerSKU))
			s.markSKUAsFailed(&sku, "Product has no variant, need to setup 'SELECT NUMBER' on TikTok first")
			continue
		}

		priceStr := fmt.Sprintf("%.0f", sku.Price)
		editSKUs = append(editSKUs, tiktok.PartialEditSKU{
			SellerSKU: sku.SellerSKU,
			SalesAttributes: []tiktok.PartialEditAttribute{
				{
					ID:        variantAttr.ID,
					Name:      variantAttr.Name,
					ValueName: sku.SellerSKU,
				},
			},
			Price: tiktok.PartialEditPrice{
				Currency:  "VND",
				Amount:    priceStr,
				SalePrice: priceStr,
			},
			Inventory: []tiktok.PartialEditInventory{
				{
					WarehouseID: warehouseID,
					Quantity:    sku.Quantity,
				},
			},
		})

		database.DB.Model(&sku).Update("sync_status", models.SKUSyncStatusPushing)
		pendingCount++

		s.log.Debug("preparing SKU for push",
			zap.String("seller_sku", sku.SellerSKU),
			zap.String("variant", variantAttr.Name))
	}

	return editSKUs, pendingCount
}

// variantAttribute holds variant info extracted from TikTok product
type variantAttribute struct {
	ID   string
	Name string
}

// getVariantAttribute gets the first variant attribute from existing SKUs
func (s *ProductSyncService) getVariantAttribute(detail *tiktok.ProductDetail) *variantAttribute {
	if len(detail.SKUs) > 0 && len(detail.SKUs[0].SalesAttributes) > 0 {
		attr := detail.SKUs[0].SalesAttributes[0]
		return &variantAttribute{
			ID:   attr.ID,
			Name: attr.Name,
		}
	}
	return nil
}

// markSKUAsFailed marks a single SKU as failed with error message
func (s *ProductSyncService) markSKUAsFailed(sku *models.SKU, errMsg string) {
	database.DB.Model(sku).Updates(map[string]interface{}{
		"sync_status":   models.SKUSyncStatusFailed,
		"error_message": errMsg,
	})
}

// markSKUsAsFailed marks multiple SKUs as failed
func (s *ProductSyncService) markSKUsAsFailed(skus []models.SKU) {
	for _, sku := range skus {
		database.DB.Model(&sku).Updates(map[string]interface{}{
			"sync_status":   models.SKUSyncStatusFailed,
			"push_attempts": sku.PushAttempts + 1,
		})
	}
}

// updatePushedSKUs updates DB with TikTok SKU IDs from response
func (s *ProductSyncService) updatePushedSKUs(tiktokProductID string, respSKUs []tiktok.PartialEditSKURes) int {
	now := time.Now()
	pushedCount := 0

	for _, resSKU := range respSKUs {
		var dbSKU models.SKU
		if err := database.DB.Where("tik_tok_product_id = ? AND seller_sku = ?", tiktokProductID, resSKU.SellerSKU).First(&dbSKU).Error; err != nil {
			continue
		}

		if dbSKU.TikTokSKUID == nil {
			database.DB.Model(&dbSKU).Updates(map[string]interface{}{
				"tik_tok_sku_id": resSKU.ID,
				"sync_status":    models.SKUSyncStatusPushed,
				"synced_at":      now,
			})
			pushedCount++

			s.log.Info("SKU pushed to TikTok",
				zap.String("seller_sku", resSKU.SellerSKU),
				zap.String("tiktok_sku_id", resSKU.ID))
		}
	}

	return pushedCount
}

// PushSingleSKUToTikTok pushes a single SKU to TikTok
func (s *ProductSyncService) PushSingleSKUToTikTok(ctx context.Context, tiktokShopID string, shopCipher string, skuID uint) error {
	var sku models.SKU
	if err := database.DB.First(&sku, skuID).Error; err != nil {
		return fmt.Errorf("SKU not found: %w", err)
	}

	if sku.TikTokSKUID != nil {
		return fmt.Errorf("SKU %s already pushed to TikTok", sku.SellerSKU)
	}

	// Look up product by TikTokProductID to get the internal product ID
	var product models.Product
	if err := database.DB.Where("tik_tok_product_id = ?", sku.TikTokProductID).First(&product).Error; err != nil {
		return fmt.Errorf("product not found for SKU %s: %w", sku.SellerSKU, err)
	}

	_, err := s.PushPendingSKUsToTikTok(ctx, tiktokShopID, shopCipher, product.ID)
	return err
}
