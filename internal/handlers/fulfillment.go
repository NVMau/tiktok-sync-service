package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	"github.com/user/sync-tiktok-mps/internal/config"
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

type ShipOrderRequest struct {
	ShopCipher     string `json:"shop_cipher"`
	TrackingNumber string `json:"tracking_number"`
	CarrierID      string `json:"carrier_id"`
}

func (h *FulfillmentHandler) ShipOrder(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	orderID := c.Params("order_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

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

	err := h.fulfillmentService.ShipOrder(c.Context(), &services.ShipOrderRequest{
		ShopID:         uint(shopID),
		ShopCipher:     req.ShopCipher,
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
	shopIDStr := c.Params("shop_id")
	orderID := c.Params("order_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	var req ShipOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	taskPayload, _ := json.Marshal(workers.ShipPayload{
		ShopID:         uint(shopID),
		ShopCipher:     req.ShopCipher,
		TikTokOrderID:  orderID,
		TrackingNumber: req.TrackingNumber,
		CarrierID:      req.CarrierID,
	})

	task := asynq.NewTask(workers.TaskShipPackage, taskPayload)
	_, err := h.asynqClient.Enqueue(task,
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
	shopIDStr := c.Params("shop_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	orders, err := h.fulfillmentService.GetOrdersReadyToShip(c.Context(), uint(shopID))
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
	shopIDStr := c.Params("shop_id")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	orders, err := h.fulfillmentService.GetOrdersInTransit(c.Context(), uint(shopID))
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

type GetShopCipherRequest struct {
	ShopCipher string `json:"shop_cipher" query:"shop_cipher"`
}

func (h *FulfillmentHandler) GetShippingProviders(c *fiber.Ctx) error {
	shopIDStr := c.Params("shop_id")
	shopCipher := c.Query("shop_cipher", "")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	providers, err := h.fulfillmentService.GetShippingProviders(c.Context(), uint(shopID), shopCipher)
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
	shopIDStr := c.Params("shop_id")
	shopCipher := c.Query("shop_cipher", "")
	shopID, _ := strconv.ParseUint(shopIDStr, 10, 32)

	warehouses, err := h.fulfillmentService.GetWarehouses(c.Context(), uint(shopID), shopCipher)
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
