package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/services"
)

type OrdersHandler struct {
	cfg              *config.Config
	orderSyncService *services.OrderSyncService
}

func NewOrdersHandler(cfg *config.Config) *OrdersHandler {
	return &OrdersHandler{
		cfg:              cfg,
		orderSyncService: services.NewOrderSyncService(),
	}
}

func (h *OrdersHandler) ListOrders(c *fiber.Ctx) error {
	shopID := c.Query("shop_id", "")
	status := c.Query("status", "")
	syncState := c.Query("sync_state", "")
	limitStr := c.Query("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	query := database.DB.Order("created_at DESC").Limit(limit)

	if shopID != "" {
		query = query.Where("shop_id = ?", shopID)
	}
	if status != "" {
		query = query.Where("tik_tok_order_status = ?", status)
	}
	if syncState != "" {
		query = query.Where("sync_state = ?", syncState)
	}

	var orders []models.Order
	if err := query.Find(&orders).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch orders",
		})
	}

	return c.JSON(fiber.Map{
		"orders": orders,
		"count":  len(orders),
	})
}

func (h *OrdersHandler) GetOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")

	var order models.Order
	if err := database.DB.First(&order, orderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	localStatus := h.orderSyncService.MapTikTokStatusToLocal(order.TikTokOrderStatus)

	return c.JSON(fiber.Map{
		"order":        order,
		"local_status": localStatus,
	})
}

func (h *OrdersHandler) SyncOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")

	var order models.Order
	if err := database.DB.First(&order, orderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "order not found",
		})
	}

	if err := h.orderSyncService.SyncOrderToLocal(c.Context(), &order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":    "order synced",
		"sync_state": order.SyncState,
	})
}

func (h *OrdersHandler) GetPendingOrders(c *fiber.Ctx) error {
	orders, err := h.orderSyncService.GetPendingSyncOrders(c.Context())
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

func (h *OrdersHandler) GetManualReviewOrders(c *fiber.Ctx) error {
	orders, err := h.orderSyncService.GetManualReviewOrders(c.Context())
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

func (h *OrdersHandler) GetOrderStats(c *fiber.Ctx) error {
	shopIDStr := c.Query("shop_id", "")

	var stats struct {
		Total           int64 `json:"total"`
		Pending         int64 `json:"pending"`
		AwaitingShipment int64 `json:"awaiting_shipment"`
		InTransit       int64 `json:"in_transit"`
		Completed       int64 `json:"completed"`
		Cancelled       int64 `json:"cancelled"`
		ManualReview    int64 `json:"manual_review"`
	}

	baseQuery := database.DB.Model(&models.Order{})
	if shopIDStr != "" {
		baseQuery = baseQuery.Where("shop_id = ?", shopIDStr)
	}

	baseQuery.Count(&stats.Total)
	database.DB.Model(&models.Order{}).Where("tik_tok_order_status = ?", models.OrderStatusUnpaid).Count(&stats.Pending)
	database.DB.Model(&models.Order{}).Where("tik_tok_order_status = ?", models.OrderStatusAwaitingShipment).Count(&stats.AwaitingShipment)
	database.DB.Model(&models.Order{}).Where("tik_tok_order_status = ?", models.OrderStatusInTransit).Count(&stats.InTransit)
	database.DB.Model(&models.Order{}).Where("tik_tok_order_status = ?", models.OrderStatusCompleted).Count(&stats.Completed)
	database.DB.Model(&models.Order{}).Where("tik_tok_order_status = ?", models.OrderStatusCancelled).Count(&stats.Cancelled)
	database.DB.Model(&models.Order{}).Where("sync_state = ?", models.SyncStateManualReview).Count(&stats.ManualReview)

	return c.JSON(stats)
}
