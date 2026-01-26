package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/services"
	"github.com/user/sync-tiktok-mps/internal/workers"
)

// =============================================================================
// InventoryHandler - Quản lý API inventory và products
// =============================================================================

type InventoryHandler struct {
	cfg              *config.Config
	inventoryService *services.InventorySyncService
	productService   *services.ProductSyncService
	asynqClient      *asynq.Client
}

func NewInventoryHandler(cfg *config.Config) *InventoryHandler {
	return &InventoryHandler{
		cfg:              cfg,
		inventoryService: services.NewInventorySyncService(cfg),
		productService:   services.NewProductSyncService(cfg),
		asynqClient:      asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL}),
	}
}

// =============================================================================
// PRODUCTS
// =============================================================================

// ListProducts - GET /api/v1/shops/:shop_id/products
func (h *InventoryHandler) ListProducts(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)
	status := c.Query("status", "")

	products, err := h.productService.ListProducts(c.Context(), uint(shopID), status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"products": products,
		"count":    len(products),
	})
}

// GetProduct - GET /api/v1/products/:id
func (h *InventoryHandler) GetProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	productID, _ := strconv.ParseUint(id, 10, 32)

	product, err := h.productService.GetProduct(c.Context(), uint(productID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "product not found",
		})
	}

	return c.JSON(fiber.Map{
		"product": product,
	})
}

// SyncProducts - POST /api/v1/shops/:shop_id/products/sync
func (h *InventoryHandler) SyncProducts(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	var req struct {
		ShopCipher string `json:"shop_cipher"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "shop_cipher required",
		})
	}

	count, err := h.productService.SyncProductsFromTikTok(c.Context(), uint(shopID), req.ShopCipher)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":      "products synced from TikTok",
		"synced_count": count,
	})
}

// =============================================================================
// SKUs
// =============================================================================

// ListSKUs - GET /api/v1/products/:product_id/skus
func (h *InventoryHandler) ListSKUs(c *fiber.Ctx) error {
	productIDStr := c.Params("product_id")
	productID, _ := strconv.ParseUint(productIDStr, 10, 32)

	syncStatus := c.Query("sync_status", "")
	saleStatus := c.Query("sale_status", "")

	skus, err := h.productService.ListSKUs(c.Context(), uint(productID), syncStatus, saleStatus)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"skus":  skus,
		"count": len(skus),
	})
}

// GetSKU - GET /api/v1/skus/:id
func (h *InventoryHandler) GetSKU(c *fiber.Ctx) error {
	id := c.Params("id")
	skuID, _ := strconv.ParseUint(id, 10, 32)

	sku, err := h.productService.GetSKU(c.Context(), uint(skuID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "SKU not found",
		})
	}

	return c.JSON(fiber.Map{
		"sku": sku,
	})
}

// CreateSKU - POST /api/v1/products/:product_id/skus
// Tạo SKU mới từ external system, chờ push lên TikTok
func (h *InventoryHandler) CreateSKU(c *fiber.Ctx) error {
	productIDStr := c.Params("product_id")
	productID, _ := strconv.ParseUint(productIDStr, 10, 32)

	var req struct {
		SellerSKU string  `json:"seller_sku"` // Số điện thoại
		Price     float64 `json:"price"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.SellerSKU == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "seller_sku required",
		})
	}

	sku, err := h.productService.CreateSKU(c.Context(), uint(productID), req.SellerSKU, req.Price)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "SKU created, pending push to TikTok",
		"sku":     sku,
	})
}

// CreateSKUsBatch - POST /api/v1/products/:product_id/skus/batch
// Tạo nhiều SKU cùng lúc
func (h *InventoryHandler) CreateSKUsBatch(c *fiber.Ctx) error {
	productIDStr := c.Params("product_id")
	productID, _ := strconv.ParseUint(productIDStr, 10, 32)

	var req struct {
		SKUs []services.SKUCreateRequest `json:"skus"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if len(req.SKUs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "skus array required",
		})
	}

	created, errors := h.productService.CreateSKUsBatch(c.Context(), uint(productID), req.SKUs)

	errorMsgs := make([]string, len(errors))
	for i, e := range errors {
		errorMsgs[i] = e.Error()
	}

	return c.JSON(fiber.Map{
		"created_count": len(created),
		"error_count":   len(errors),
		"created":       created,
		"errors":        errorMsgs,
	})
}

// =============================================================================
// INVENTORY SYNC
// =============================================================================

// SyncSKUInventory - POST /api/v1/shops/:shop_id/skus/:sku_id/sync
func (h *InventoryHandler) SyncSKUInventory(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	skuIDStr := c.Params("sku_id")

	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)
	skuID, _ := strconv.ParseUint(skuIDStr, 10, 32)

	var req struct {
		ShopCipher string `json:"shop_cipher"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "shop_cipher required",
		})
	}

	err := h.inventoryService.SyncSKUInventoryToTikTok(c.Context(), uint(shopID), req.ShopCipher, uint(skuID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "SKU inventory synced to TikTok",
	})
}

// SyncProductInventory - POST /api/v1/shops/:shop_id/products/:product_id/sync-inventory
func (h *InventoryHandler) SyncProductInventory(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	productIDStr := c.Params("product_id")

	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)
	productID, _ := strconv.ParseUint(productIDStr, 10, 32)

	var req struct {
		ShopCipher string `json:"shop_cipher"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "shop_cipher required",
		})
	}

	// Enqueue async task
	taskPayload, _ := json.Marshal(workers.SyncAllInventoryPayload{
		ShopID:     uint(shopID),
		ShopCipher: req.ShopCipher,
		ProductID:  uint(productID),
	})

	task := asynq.NewTask(workers.TaskSyncInventory, taskPayload)
	_, err := h.asynqClient.Enqueue(task, asynq.Queue("default"), asynq.MaxRetry(3))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to enqueue sync task",
		})
	}

	return c.JSON(fiber.Map{
		"message":    "inventory sync task enqueued",
		"product_id": productID,
	})
}

// =============================================================================
// STATISTICS
// =============================================================================

// GetInventoryStats - GET /api/v1/shops/:shop_id/inventory/stats
func (h *InventoryHandler) GetInventoryStats(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	stats, err := h.inventoryService.GetInventoryStats(c.Context(), uint(shopID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(stats)
}

// GetPendingSKUs - GET /api/v1/shops/:shop_id/skus/pending
func (h *InventoryHandler) GetPendingSKUs(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	skus, err := h.productService.GetPendingSKUs(c.Context(), uint(shopID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"pending_skus": skus,
		"count":        len(skus),
	})
}

// =============================================================================
// LEGACY ENDPOINTS (for backward compatibility)
// =============================================================================

// ListInventory - GET /api/v1/inventory (legacy)
func (h *InventoryHandler) ListInventory(c *fiber.Ctx) error {
	shopIDStr := c.Query("shop_id", "")
	saleStatus := c.Query("sale_status", "")
	limitStr := c.Query("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	query := database.DB.Preload("Product").Order("updated_at DESC").Limit(limit)

	if shopIDStr != "" {
		query = query.Joins("JOIN tiktok_sync.products ON products.id = skus.product_id").
			Where("products.shop_id = ?", shopIDStr)
	}
	if saleStatus != "" {
		query = query.Where("sale_status = ?", saleStatus)
	}

	var skus []models.SKU
	if err := query.Find(&skus).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch inventory",
		})
	}

	return c.JSON(fiber.Map{
		"inventory": skus,
		"count":     len(skus),
	})
}

// GetInventory - GET /api/v1/inventory/:id (legacy)
func (h *InventoryHandler) GetInventory(c *fiber.Ctx) error {
	id := c.Params("id")
	skuID, _ := strconv.ParseUint(id, 10, 32)

	sku, err := h.productService.GetSKU(c.Context(), uint(skuID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "inventory not found",
		})
	}

	return c.JSON(fiber.Map{
		"inventory": sku,
	})
}

// UpdateInventory - PUT /api/v1/skus/:id/price (update SKU price)
func (h *InventoryHandler) UpdateInventory(c *fiber.Ctx) error {
	skuIDStr := c.Params("id")
	skuID, _ := strconv.ParseUint(skuIDStr, 10, 32)

	var req struct {
		Price float64 `json:"price"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	err := h.productService.UpdateSKUPrice(c.Context(), uint(skuID), req.Price)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "SKU price updated",
	})
}

// SyncInventory - POST /api/v1/shops/:shop_id/inventory/:sim_id/sync (legacy - redirect to SKU sync)
func (h *InventoryHandler) SyncInventory(c *fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error":   "deprecated endpoint",
		"message": "use POST /api/v1/shops/:shop_id/skus/:sku_id/sync instead",
	})
}

// SyncAllDirty - POST /api/v1/shops/:shop_id/inventory/sync-all (legacy)
func (h *InventoryHandler) SyncAllDirty(c *fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error":   "deprecated endpoint",
		"message": "use POST /api/v1/shops/:shop_id/products/:product_id/sync-inventory instead",
	})
}

// ListMappings - GET /api/v1/mappings (legacy - redirect to products)
func (h *InventoryHandler) ListMappings(c *fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error":   "deprecated endpoint",
		"message": "use GET /api/v1/shops/:shop_id/products instead",
	})
}

// CreateMapping - POST /api/v1/shops/:shop_id/mappings (legacy - redirect to SKU create)
func (h *InventoryHandler) CreateMapping(c *fiber.Ctx) error {
	return c.Status(fiber.StatusGone).JSON(fiber.Map{
		"error":   "deprecated endpoint",
		"message": "use POST /api/v1/products/:product_id/skus instead",
	})
}
