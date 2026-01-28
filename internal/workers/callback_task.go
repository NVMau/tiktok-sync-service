package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/services"
)

// CallbackType defines the type of callback
type CallbackType string

const (
	CallbackTypeOrderCreated      CallbackType = "ORDER_CREATED"
	CallbackTypeOrderStatusChange CallbackType = "ORDER_STATUS_CHANGE"
)

// CallbackTaskPayload contains data for callback task
type CallbackTaskPayload struct {
	Type          CallbackType `json:"type"`
	TikTokOrderID string       `json:"tiktok_order_id"`
	TikTokShopID  string       `json:"tiktok_shop_id"`
	OldStatus     string       `json:"old_status,omitempty"`
	NewStatus     string       `json:"new_status,omitempty"`
}

// EnqueueOrderCreatedCallback enqueues a callback task for new order
func EnqueueOrderCreatedCallback(client *asynq.Client, tiktokOrderID, tiktokShopID string) error {
	payload := CallbackTaskPayload{
		Type:          CallbackTypeOrderCreated,
		TikTokOrderID: tiktokOrderID,
		TikTokShopID:  tiktokShopID,
	}
	return enqueueCallbackTask(client, payload)
}

// EnqueueOrderStatusChangeCallback enqueues a callback task for status change
func EnqueueOrderStatusChangeCallback(client *asynq.Client, tiktokOrderID, tiktokShopID, oldStatus, newStatus string) error {
	payload := CallbackTaskPayload{
		Type:          CallbackTypeOrderStatusChange,
		TikTokOrderID: tiktokOrderID,
		TikTokShopID:  tiktokShopID,
		OldStatus:     oldStatus,
		NewStatus:     newStatus,
	}
	return enqueueCallbackTask(client, payload)
}

func enqueueCallbackTask(client *asynq.Client, payload CallbackTaskPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal callback payload: %w", err)
	}

	task := asynq.NewTask(TaskSendCallback, data)
	opts := []asynq.Option{
		asynq.MaxRetry(5),
		asynq.Queue("default"),
	}

	_, err = client.Enqueue(task, opts...)
	if err != nil {
		logger.Error("failed to enqueue callback task",
			zap.Error(err),
			zap.String("type", string(payload.Type)),
			zap.String("order_id", payload.TikTokOrderID))
		return err
	}

	logger.Debug("callback task enqueued",
		zap.String("type", string(payload.Type)),
		zap.String("order_id", payload.TikTokOrderID))
	return nil
}

// HandleSendCallback processes callback tasks
func HandleSendCallback(ctx context.Context, task *asynq.Task) error {
	var payload CallbackTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to parse callback task payload: %w", err)
	}

	cfg := config.Get()
	callbackService := services.NewCallbackService(cfg)

	if !callbackService.IsEnabled() {
		logger.Debug("callback disabled, skipping task")
		return nil
	}

	switch payload.Type {
	case CallbackTypeOrderCreated:
		return handleOrderCreatedCallback(ctx, callbackService, payload)
	case CallbackTypeOrderStatusChange:
		return handleOrderStatusChangeCallback(ctx, callbackService, payload)
	default:
		logger.Warn("unknown callback type", zap.String("type", string(payload.Type)))
		return nil
	}
}

func handleOrderCreatedCallback(ctx context.Context, svc *services.CallbackService, payload CallbackTaskPayload) error {
	var order models.Order
	if err := database.DB.Where("tik_tok_order_id = ?", payload.TikTokOrderID).First(&order).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	var items []models.OrderItem
	if err := database.DB.Where("tik_tok_order_id = ?", payload.TikTokOrderID).Find(&items).Error; err != nil {
		return fmt.Errorf("failed to fetch order items: %w", err)
	}

	if err := svc.SendOrderCreatedCallback(ctx, &order, items); err != nil {
		return fmt.Errorf("failed to send order created callback: %w", err)
	}

	logger.Info("order created callback sent",
		zap.String("tiktok_order_id", payload.TikTokOrderID),
		zap.Int("items_count", len(items)))
	return nil
}

func handleOrderStatusChangeCallback(ctx context.Context, svc *services.CallbackService, payload CallbackTaskPayload) error {
	if err := svc.SendOrderStatusChangeCallback(ctx, payload.TikTokOrderID, payload.TikTokShopID, payload.OldStatus, payload.NewStatus); err != nil {
		return fmt.Errorf("failed to send status change callback: %w", err)
	}

	logger.Info("order status change callback sent",
		zap.String("tiktok_order_id", payload.TikTokOrderID),
		zap.String("old_status", payload.OldStatus),
		zap.String("new_status", payload.NewStatus))
	return nil
}
