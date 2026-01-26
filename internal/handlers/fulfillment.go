package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/services"
	"github.com/user/sync-tiktok-mps/internal/workers"
)

type FulfillmentHandler struct {
	cfg                *config.Config
	fulfillmentService *services.FulfillmentSyncService
	asynqClient        *asynq.Client
}

func NewFulfillmentHandler(cfg *config.Config) *FulfillmentHandler {
	return &FulfillmentHandler{
		cfg:                cfg,
		fulfillmentService: services.NewFulfillmentSyncService(cfg),
		asynqClient:        asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL}),
	}
}

// getShopByParam looks up shop by TikTok Shop ID or auto-increment ID
func (h *FulfillmentHandler) getShopByParam(shopIDParam string) (*models.Shop, error) {
	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", shopIDParam).First(&shop).Error; err == nil {
		return &shop, nil
	}
	if id, err := strconv.ParseUint(shopIDParam, 10, 32); err == nil {
		if err := database.DB.First(&shop, id).Error; err == nil {
			return &shop, nil
		}
	}
	return nil, fiber.NewError(fiber.StatusNotFound, "shop not found")
}

type ShipOrderRequest struct {
	ShopCipher     string `json:"shop_cipher"`
	TrackingNumber string `json:"tracking_number"`
	CarrierID      string `json:"carrier_id"`
}

func (h *FulfillmentHandler) ShipOrder(c *fiber.Ctx) error {
	shop, err := h.getShopByParam(c.Params("shop_id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "shop not found"})
	}
	orderID := c.Params("order_id")

	var req ShipOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.TrackingNumber == "" || req.CarrierID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tracking_number and carrier_id required",
		})
	}

	shopCipher := req.ShopCipher
	if shopCipher == "" {
		shopCipher = shop.ShopCipher
	}

	err = h.fulfillmentService.ShipOrder(c.Context(), &services.ShipOrderRequest{
		ShopID:         shop.ID,
		ShopCipher:     shopCipher,
		TikTokOrderID:  orderID,
		TrackingNumber: req.TrackingNumber,
		CarrierID:      req.CarrierID,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":         "order shipped",
		"tracking_number": req.TrackingNumber,
	})
}

func (h *FulfillmentHandler) ShipOrderAsync(c *fiber.Ctx) error {
	shop, err := h.getShopByParam(c.Params("shop_id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "shop not found"})
	}
	orderID := c.Params("order_id")

	var req ShipOrderRequest
	_ = c.BodyParser(&req)

	shopCipher := req.ShopCipher
	if shopCipher == "" {
		shopCipher = shop.ShopCipher
	}

	taskPayload, _ := json.Marshal(workers.ShipPayload{
		ShopID:         shop.ID,
		ShopCipher:     shopCipher,
		TikTokOrderID:  orderID,
		TrackingNumber: req.TrackingNumber,
		CarrierID:      req.CarrierID,
	})

	task := asynq.NewTask(workers.TaskShipPackage, taskPayload)
	_, err = h.asynqClient.Enqueue(task,
		asynq.Queue("critical"),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to enqueue ship task",
		})
	}

	return c.JSON(fiber.Map{
		"message": "ship task enqueued",
	})
}

func (h *FulfillmentHandler) GetReadyToShipOrders(c *fiber.Ctx) error {
	shop, err := h.getShopByParam(c.Params("shop_id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "shop not found"})
	}

	orders, err := h.fulfillmentService.GetOrdersReadyToShip(c.Context(), shop.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"orders": orders,
		"count":  len(orders),
	})
}

func (h *FulfillmentHandler) GetInTransitOrders(c *fiber.Ctx) error {
	shop, err := h.getShopByParam(c.Params("shop_id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "shop not found"})
	}

	orders, err := h.fulfillmentService.GetOrdersInTransit(c.Context(), shop.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"orders": orders,
		"count":  len(orders),
	})
}

func (h *FulfillmentHandler) GetShippingProviders(c *fiber.Ctx) error {
	shop, err := h.getShopByParam(c.Params("shop_id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "shop not found"})
	}
	shopCipher := c.Query("shop_cipher", "")
	if shopCipher == "" {
		shopCipher = shop.ShopCipher
	}

	providers, err := h.fulfillmentService.GetShippingProviders(c.Context(), shop.ID, shopCipher)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"providers": providers,
		"count":     len(providers),
	})
}

func (h *FulfillmentHandler) GetWarehouses(c *fiber.Ctx) error {
	shop, err := h.getShopByParam(c.Params("shop_id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "shop not found"})
	}
	shopCipher := c.Query("shop_cipher", "")
	if shopCipher == "" {
		shopCipher = shop.ShopCipher
	}

	warehouses, err := h.fulfillmentService.GetWarehouses(c.Context(), shop.ID, shopCipher)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"warehouses": warehouses,
		"count":      len(warehouses),
	})
}
