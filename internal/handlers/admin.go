package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

type AdminHandler struct {
	cfg          *config.Config
	authClient   *tiktok.AuthClient
	tokenManager *tiktok.TokenManager
	client       *tiktok.Client
	shopsAPI     *tiktok.ShopsAPI
}

func NewAdminHandler(cfg *config.Config) *AdminHandler {
	client := tiktok.NewClient(cfg)
	tokenManager := tiktok.NewTokenManager(cfg)

	return &AdminHandler{
		cfg:          cfg,
		authClient:   tiktok.NewAuthClient(cfg),
		tokenManager: tokenManager,
		client:       client,
		shopsAPI:     tiktok.NewShopsAPI(client, tokenManager),
	}
}

type AuthCallbackRequest struct {
	AuthCode string `json:"auth_code" query:"code"`
	State    string `json:"state" query:"state"`
}

func (h *AdminHandler) HandleAuthCallback(c *fiber.Ctx) error {
	var req AuthCallbackRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	if req.AuthCode == "" || req.AuthCode == "null" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "authorization denied or auth_code missing",
		})
	}

	tokenResp, err := h.authClient.GetAccessToken(c.Context(), req.AuthCode)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	shops, err := h.shopsAPI.GetAuthorizedShops(c.Context(), tokenResp.Data.AccessToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to get authorized shops",
			"details": err.Error(),
		})
	}

	var savedShops []models.Shop
	for _, shop := range shops {
		shopModel := models.Shop{
			ShopID:     shop.ID,
			ShopCipher: shop.Cipher,
			ShopName:   shop.Name,
			Region:     shop.Region,
			Status:     "active",
		}

		result := database.DB.Where("shop_id = ?", shop.ID).
			Assign(shopModel).
			FirstOrCreate(&shopModel)

		if result.Error != nil {
			continue
		}

		if err := h.tokenManager.SaveToken(c.Context(), shopModel.ShopID, tokenResp); err != nil {
			continue
		}

		savedShops = append(savedShops, shopModel)
	}

	return c.JSON(fiber.Map{
		"message": "authorization successful",
		"shops":   savedShops,
		"seller":  tokenResp.Data.SellerName,
		"region":  tokenResp.Data.SellerBaseRegion,
		"scopes":  tokenResp.Data.GrantedScopes,
	})
}

func (h *AdminHandler) GetAuthorizationURL(c *fiber.Ctx) error {
	state := c.Query("state", "default_state")

	serviceID := h.cfg.TikTokServiceID
	if serviceID == "" {
		serviceID = h.cfg.TikTokAppKey
	}
	authURL := "https://services.tiktokshop.com/open/authorize?service_id=" + serviceID + "&state=" + state

	return c.JSON(fiber.Map{
		"authorization_url": authURL,
		"instructions": "Visit this URL to authorize your TikTok Shop. After authorization, you will be redirected to your callback URL with an auth_code.",
	})
}

func (h *AdminHandler) ListShops(c *fiber.Ctx) error {
	var shops []models.Shop
	if err := database.DB.Find(&shops).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list shops",
		})
	}

	return c.JSON(fiber.Map{
		"shops": shops,
	})
}

func (h *AdminHandler) GetShopDetail(c *fiber.Ctx) error {
	shopID := c.Params("id")

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopID).First(&shop).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "shop not found",
		})
	}

	var token models.OAuthToken
	database.DB.Where("tik_tok_shop_id = ?", shop.ShopID).First(&token)

	return c.JSON(fiber.Map{
		"shop": shop,
		"token": fiber.Map{
			"expires_at": token.ExpiresAt,
			"scopes":     token.Scopes,
			"updated_at": token.UpdatedAt,
		},
	})
}

func (h *AdminHandler) RefreshShopToken(c *fiber.Ctx) error {
	shopID := c.Params("id")

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopID).First(&shop).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "shop not found",
		})
	}

	accessToken, err := h.tokenManager.GetValidToken(c.Context(), shop.ShopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":      "token refreshed successfully",
		"access_token": accessToken[:20] + "...",
	})
}

type TestAPIRequest struct {
	ShopCipher string `json:"shop_cipher"`
}

func (h *AdminHandler) TestShopConnection(c *fiber.Ctx) error {
	shopID := c.Params("id")

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopID).First(&shop).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "shop not found",
		})
	}

	var req TestAPIRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "shop_cipher is required in body",
		})
	}

	logisticsAPI := tiktok.NewLogisticsAPI(h.client, h.tokenManager)
	warehouses, err := logisticsAPI.GetWarehouses(c.Context(), shop.ShopID, req.ShopCipher)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "API test failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":    "connection successful",
		"warehouses": warehouses,
	})
}

func (h *AdminHandler) GetProducts(c *fiber.Ctx) error {
	shopID := c.Params("shop_id")

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopID).First(&shop).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "shop not found",
		})
	}

	if shop.ShopCipher == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "shop_cipher not found, please re-authorize the shop",
		})
	}

	productsAPI := tiktok.NewProductsAPI(h.client, h.tokenManager)
	products, err := productsAPI.GetProductList(c.Context(), &tiktok.ProductListRequest{
		TikTokShopID: shop.ShopID,
		ShopCipher:   shop.ShopCipher,
		PageSize:     20,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to fetch products",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"products":    products.Products,
		"total_count": products.TotalCount,
	})
}

func (h *AdminHandler) SyncOrders(c *fiber.Ctx) error {
	shopID := c.Params("shop_id")

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopID).First(&shop).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "shop not found",
		})
	}

	if shop.ShopCipher == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "shop_cipher not found, please re-authorize the shop",
		})
	}

	// Get orders from last 7 days by default
	days := c.QueryInt("days", 7)
	since := time.Duration(days) * 24 * time.Hour

	ordersAPI := tiktok.NewOrdersAPI(h.client, h.tokenManager)
	orders, err := ordersAPI.GetRecentOrders(c.Context(), shop.ShopID, shop.ShopCipher, since)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to fetch orders from TikTok",
			"details": err.Error(),
		})
	}

	// Sync to database
	var synced, updated, failed int
	var lastError string
	for _, order := range orders {
		// Get order details
		detail, err := ordersAPI.GetOrderDetail(c.Context(), shop.ShopID, shop.ShopCipher, order.ID)
		if err != nil {
			lastError = err.Error()
			failed++
			continue
		}

		// Parse total amount
		totalAmount := 0.0
		if detail.Payment.TotalAmount != "" {
			totalAmount, _ = strconv.ParseFloat(detail.Payment.TotalAmount, 64)
		}

		// Prepare JSON fields
		recipientJSON, _ := json.Marshal(detail.RecipientAddress)
		rawPayload, _ := json.Marshal(detail)

		// Convert timestamps
		var placedAt, paidAt *time.Time
		if detail.CreateTime > 0 {
			t := time.Unix(detail.CreateTime, 0)
			placedAt = &t
		}
		if detail.PaidTime > 0 {
			t := time.Unix(detail.PaidTime, 0)
			paidAt = &t
		}

		orderModel := models.Order{
			TikTokShopID:      shop.ShopID,
			TikTokOrderID:     order.ID,
			TikTokOrderStatus: models.TikTokOrderStatus(detail.Status),
			PaymentStatus:     detail.PaymentMethodName,
			ShippingAddress:   recipientJSON,
			TotalAmount:       totalAmount,
			Currency:          detail.Payment.Currency,
			PlacedAt:          placedAt,
			PaidAt:            paidAt,
			RawPayload:        rawPayload,
			SyncState:         models.SyncStateNew,
		}

		// Upsert order
		var existing models.Order
		result := database.DB.Where("tik_tok_order_id = ?", order.ID).First(&existing)
		if result.Error != nil {
			// New order
			if err := database.DB.Create(&orderModel).Error; err != nil {
				failed++
				continue
			}
			synced++
		} else {
			// Update existing
			orderModel.ID = existing.ID
			orderModel.SyncState = existing.SyncState // Keep existing sync state
			if err := database.DB.Save(&orderModel).Error; err != nil {
				failed++
				continue
			}
			updated++
		}

		// Sync order items
		for _, item := range detail.LineItems {
			price := 0.0
			if item.SalePrice != "" {
				price, _ = strconv.ParseFloat(item.SalePrice, 64)
			}

			itemModel := models.OrderItem{
				TikTokOrderID:     order.ID,
				TikTokOrderItemID: item.ID,
				TikTokProductID:   item.ProductID,
				TikTokSKUID:       item.SkuID,
				SellerSKU:         item.SellerSku,
				Qty:               item.Quantity,
				Price:             price,
			}

			if err := database.DB.Where("tik_tok_order_item_id = ?", item.ID).
				Assign(itemModel).
				FirstOrCreate(&itemModel).Error; err != nil {
				logger.Error("failed to save order item",
					zap.String("item_id", item.ID),
					zap.Error(err))
			}
		}
	}

	result := fiber.Map{
		"message":       "sync completed",
		"total_fetched": len(orders),
		"synced":        synced,
		"updated":       updated,
		"failed":        failed,
	}
	if lastError != "" {
		result["last_error"] = lastError
	}
	return c.JSON(result)
}

func (h *AdminHandler) GetOrders(c *fiber.Ctx) error {
	shopID := c.Params("shop_id")

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopID).First(&shop).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "shop not found",
		})
	}

	ordersAPI := tiktok.NewOrdersAPI(h.client, h.tokenManager)
	orders, err := ordersAPI.GetOrderList(c.Context(), &tiktok.OrderListRequest{
		TikTokShopID: shop.ShopID,
		ShopCipher:   shop.ShopCipher,
		PageSize:     20,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to fetch orders",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"orders":      orders.Orders,
		"total_count": orders.TotalCount,
	})
}
