package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
)

// CallbackEventType defines the type of callback event
type CallbackEventType string

const (
	CallbackEventOrderCreated       CallbackEventType = "ORDER_CREATED"
	CallbackEventOrderStatusChange  CallbackEventType = "ORDER_STATUS_CHANGE"
)

// CallbackItemPayload represents a single order item in callback
type CallbackItemPayload struct {
	TikTokOrderItemID string  `json:"tiktok_order_item_id"`
	TikTokSKUID       string  `json:"tiktok_sku_id"`
	SellerSKU         string  `json:"seller_sku"`
	Qty               int     `json:"qty"`
	Price             float64 `json:"price"`
}

// OrderCreatedCallback payload for new order
type OrderCreatedCallback struct {
	EventType         CallbackEventType     `json:"event_type"`
	TikTokOrderID     string                `json:"tiktok_order_id"`
	TikTokShopID      string                `json:"tiktok_shop_id"`
	TikTokOrderStatus string                `json:"tiktok_order_status"`
	TotalAmount       float64               `json:"total_amount"`
	Currency          string                `json:"currency"`
	PlacedAt          *time.Time            `json:"placed_at"`
	Items             []CallbackItemPayload `json:"items"`
	Timestamp         int64                 `json:"timestamp"`
}

// OrderStatusChangeCallback payload for status change
type OrderStatusChangeCallback struct {
	EventType     CallbackEventType `json:"event_type"`
	TikTokOrderID string            `json:"tiktok_order_id"`
	TikTokShopID  string            `json:"tiktok_shop_id"`
	OldStatus     string            `json:"old_status"`
	NewStatus     string            `json:"new_status"`
	Timestamp     int64             `json:"timestamp"`
}

// CallbackService handles sending callbacks to external services
type CallbackService struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewCallbackService creates a new callback service
func NewCallbackService(cfg *config.Config) *CallbackService {
	return &CallbackService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.CallbackTimeout) * time.Second,
		},
	}
}

// IsEnabled checks if callback is configured
func (s *CallbackService) IsEnabled() bool {
	return s.cfg.CallbackURL != ""
}

// SendOrderCreatedCallback sends callback for new order
func (s *CallbackService) SendOrderCreatedCallback(ctx context.Context, order *models.Order, items []models.OrderItem) error {
	if !s.IsEnabled() {
		logger.Debug("callback disabled, skipping order created callback")
		return nil
	}

	// Build items payload with seller_sku from SKU table
	callbackItems := make([]CallbackItemPayload, 0, len(items))
	for _, item := range items {
		sellerSKU := s.getSellerSKU(item.TikTokSKUID)
		callbackItems = append(callbackItems, CallbackItemPayload{
			TikTokOrderItemID: item.TikTokOrderItemID,
			TikTokSKUID:       item.TikTokSKUID,
			SellerSKU:         sellerSKU,
			Qty:               item.Qty,
			Price:             item.Price,
		})
	}

	payload := OrderCreatedCallback{
		EventType:         CallbackEventOrderCreated,
		TikTokOrderID:     order.TikTokOrderID,
		TikTokShopID:      order.TikTokShopID,
		TikTokOrderStatus: string(order.TikTokOrderStatus),
		TotalAmount:       order.TotalAmount,
		Currency:          order.Currency,
		PlacedAt:          order.PlacedAt,
		Items:             callbackItems,
		Timestamp:         time.Now().Unix(),
	}

	logger.Debug("order created callback payload",
		zap.String("tiktok_order_id", order.TikTokOrderID),
		zap.String("tiktok_shop_id", order.TikTokShopID),
		zap.String("status", string(order.TikTokOrderStatus)),
		zap.Float64("total_amount", order.TotalAmount),
		zap.Int("items_count", len(callbackItems)))

	return s.sendCallback(ctx, payload)
}

// SendOrderStatusChangeCallback sends callback for order status change
func (s *CallbackService) SendOrderStatusChangeCallback(ctx context.Context, tiktokOrderID, tiktokShopID, oldStatus, newStatus string) error {
	if !s.IsEnabled() {
		logger.Debug("callback disabled, skipping status change callback")
		return nil
	}

	payload := OrderStatusChangeCallback{
		EventType:     CallbackEventOrderStatusChange,
		TikTokOrderID: tiktokOrderID,
		TikTokShopID:  tiktokShopID,
		OldStatus:     oldStatus,
		NewStatus:     newStatus,
		Timestamp:     time.Now().Unix(),
	}

	logger.Debug("order status change callback payload",
		zap.String("tiktok_order_id", tiktokOrderID),
		zap.String("tiktok_shop_id", tiktokShopID),
		zap.String("old_status", oldStatus),
		zap.String("new_status", newStatus))

	return s.sendCallback(ctx, payload)
}

// sendCallback sends the callback with retry logic
func (s *CallbackService) sendCallback(ctx context.Context, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal callback payload: %w", err)
	}

	logger.Debug("preparing callback request",
		zap.String("url", s.cfg.CallbackURL),
		zap.Int("payload_size", len(jsonData)))

	var lastErr error
	for attempt := 0; attempt <= s.cfg.CallbackRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s...
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			logger.Debug("retrying callback",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.CallbackURL, bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Callback-Source", "tiktok-sync-server")
		req.Header.Set("X-Callback-Timestamp", timestamp)
		
		// Add HMAC signature if secret is configured
		if s.cfg.CallbackSecret != "" {
			signature := s.generateSignature(jsonData, timestamp)
			req.Header.Set("X-Callback-Signature", signature)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("callback request failed: %w", err)
			logger.Warn("callback request failed",
				zap.Error(err),
				zap.Int("attempt", attempt),
				zap.String("url", s.cfg.CallbackURL))
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.Info("callback sent successfully",
				zap.String("url", s.cfg.CallbackURL),
				zap.Int("status", resp.StatusCode),
				zap.Int("attempts", attempt+1))
			return nil
		}

		lastErr = fmt.Errorf("callback returned status %d", resp.StatusCode)
		logger.Warn("callback returned non-success status",
			zap.Int("status", resp.StatusCode),
			zap.Int("attempt", attempt))
	}

	logger.Error("callback failed after all retries",
		zap.Error(lastErr),
		zap.Int("max_retries", s.cfg.CallbackRetries),
		zap.String("url", s.cfg.CallbackURL))
	return lastErr
}

// generateSignature creates HMAC-SHA256 signature for callback verification
// Signature format: HMAC-SHA256(timestamp + "." + body, secret)
func (s *CallbackService) generateSignature(body []byte, timestamp string) string {
	message := timestamp + "." + string(body)
	h := hmac.New(sha256.New, []byte(s.cfg.CallbackSecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// getSellerSKU retrieves seller_sku from SKU table
func (s *CallbackService) getSellerSKU(tiktokSKUID string) string {
	if tiktokSKUID == "" {
		return ""
	}

	var sku models.SKU
	if err := database.DB.Where("tik_tok_sku_id = ?", tiktokSKUID).First(&sku).Error; err != nil {
		logger.Debug("SKU not found for seller_sku lookup", zap.String("tiktok_sku_id", tiktokSKUID))
		return ""
	}
	return sku.SellerSKU
}
