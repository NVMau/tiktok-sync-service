package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/workers"
	"github.com/user/sync-tiktok-mps/pkg/signature"
)

type WebhookHandler struct {
	cfg         *config.Config
	asynqClient *asynq.Client
}

func NewWebhookHandler(cfg *config.Config) *WebhookHandler {
	return &WebhookHandler{
		cfg:         cfg,
		asynqClient: asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL}),
	}
}

type WebhookPayload struct {
	Type              int             `json:"type"`
	TTSNotificationID string          `json:"tts_notification_id"`
	ShopID            string          `json:"shop_id"`
	Timestamp         int64           `json:"timestamp"`
	Data              json.RawMessage `json:"data"`
}

// Webhook type constants
const (
	WebhookTypeOrderStatusChange    = 1
	WebhookTypeReverseStatusUpdate  = 2
	WebhookTypeRecipientAddress     = 3
	WebhookTypePackageUpdate        = 4
	WebhookTypeProductStatusChange  = 5
	WebhookTypeSellerDeauthorization = 6
	WebhookTypeAuthExpire           = 7
)

func getWebhookTypeName(typeCode int) string {
	switch typeCode {
	case WebhookTypeOrderStatusChange:
		return "ORDER_STATUS_CHANGE"
	case WebhookTypeReverseStatusUpdate:
		return "REVERSE_STATUS_UPDATE"
	case WebhookTypeRecipientAddress:
		return "RECIPIENT_ADDRESS_UPDATE"
	case WebhookTypePackageUpdate:
		return "PACKAGE_UPDATE"
	case WebhookTypeProductStatusChange:
		return "PRODUCT_STATUS_CHANGE"
	case WebhookTypeSellerDeauthorization:
		return "SELLER_DEAUTHORIZATION"
	case WebhookTypeAuthExpire:
		return "AUTH_EXPIRE"
	default:
		return fmt.Sprintf("UNKNOWN_%d", typeCode)
	}
}

func (h *WebhookHandler) HandleTikTokWebhook(c *fiber.Ctx) error {
	body := c.Body()

	logger.Debug("webhook raw body received", zap.ByteString("body", body))

	sigValid, sigTimestamp := h.verifySignature(c, body)

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("webhook parse error",
			zap.Error(err),
			zap.ByteString("body", body))
		return c.SendStatus(fiber.StatusOK)
	}

	if !h.validateTimestamp(payload.Timestamp) {
		logger.Warn("webhook timestamp too old", zap.Int64("timestamp", payload.Timestamp))
	}

	var shop models.Shop
	if err := database.DB.Where("shop_id = ?", payload.ShopID).First(&shop).Error; err != nil {
		logger.Warn("webhook received for unknown shop", zap.String("shop_id", payload.ShopID))
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "shop not found, ignored",
		})
	}

	eventTypeName := getWebhookTypeName(payload.Type)
	eventID := generateEventID(payload.ShopID, eventTypeName, payload.Timestamp, body)

	event := models.WebhookEvent{
		ShopID:         shop.ID,
		EventID:        eventID,
		EventType:      eventTypeName,
		ReceivedAt:     time.Now(),
		Payload:        body,
		SignatureValid: sigValid,
		ProcessStatus:  models.EventStatusPending,
	}

	result := database.DB.Where("event_id = ?", eventID).FirstOrCreate(&event)
	if result.Error != nil {
		logger.Error("failed to store webhook event", zap.Error(result.Error))
		return c.SendStatus(fiber.StatusOK)
	}

	if result.RowsAffected == 0 {
		logger.Debug("duplicate webhook event", zap.String("event_id", eventID))
		return c.SendStatus(fiber.StatusOK)
	}

	if err := h.enqueueProcessing(event.ID, eventTypeName); err != nil {
		logger.Error("failed to enqueue webhook processing", zap.Error(err))
		event.ProcessStatus = models.EventStatusFailed
		event.Error = err.Error()
		database.DB.Save(&event)
	}

	logger.Info("webhook received",
		zap.String("event_type", eventTypeName),
		zap.String("shop_id", payload.ShopID),
		zap.String("event_id", eventID),
		zap.Bool("sig_valid", sigValid),
		zap.String("sig_timestamp", sigTimestamp))

	// TikTok requires 200 with empty body
	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) verifySignature(c *fiber.Ctx, body []byte) (bool, string) {
	sigHeader := c.Get("Tiktok-Signature")
	if sigHeader == "" {
		sigHeader = c.Get("X-Tiktok-Signature")
	}

	if sigHeader == "" {
		return false, ""
	}

	timestamp, sig := signature.ParseWebhookSignatureHeader(sigHeader)
	if timestamp == "" || sig == "" {
		return false, ""
	}

	if h.cfg.TikTokWebhookSecret == "" {
		return true, timestamp
	}

	valid := signature.VerifyWebhookSignature(h.cfg.TikTokWebhookSecret, timestamp, body, sig)
	return valid, timestamp
}

func (h *WebhookHandler) validateTimestamp(ts int64) bool {
	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	return diff <= 300
}

func (h *WebhookHandler) enqueueProcessing(eventID uint, eventType string) error {
	taskPayload, _ := json.Marshal(workers.WebhookTaskPayload{
		EventID:   eventID,
		EventType: eventType,
	})

	task := asynq.NewTask(workers.TaskProcessWebhook, taskPayload)

	opts := []asynq.Option{
		asynq.MaxRetry(5),
		asynq.Queue("default"),
		asynq.Timeout(30 * time.Second),
	}

	if eventType == "ORDER_STATUS_CHANGE" {
		opts = append(opts, asynq.Queue("critical"))
	}

	_, err := h.asynqClient.Enqueue(task, opts...)
	return err
}

func generateEventID(shopID, eventType string, timestamp int64, body []byte) string {
	data := fmt.Sprintf("%s:%s:%d:%s", shopID, eventType, timestamp, string(body))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

func HandleTikTokWebhook(c *fiber.Ctx) error {
	cfg := config.Get()
	handler := NewWebhookHandler(cfg)
	return handler.HandleTikTokWebhook(c)
}

func (h *WebhookHandler) GetWebhookEvents(c *fiber.Ctx) error {
	status := c.Query("status", "")
	limitStr := c.Query("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	var events []models.WebhookEvent
	query := database.DB.Order("received_at DESC").Limit(limit)

	if status != "" {
		query = query.Where("process_status = ?", status)
	}

	if err := query.Find(&events).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch events",
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"count":  len(events),
	})
}

func (h *WebhookHandler) RetryWebhookEvent(c *fiber.Ctx) error {
	eventID := c.Params("id")

	var event models.WebhookEvent
	if err := database.DB.First(&event, eventID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "event not found",
		})
	}

	event.ProcessStatus = models.EventStatusPending
	event.Error = ""
	event.ProcessedAt = nil
	database.DB.Save(&event)

	if err := h.enqueueProcessing(event.ID, event.EventType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to enqueue",
		})
	}

	return c.JSON(fiber.Map{
		"message":  "event requeued",
		"event_id": event.EventID,
	})
}
