package models

import (
	"testing"
	"time"
)

func TestShop_TableName(t *testing.T) {
	shop := Shop{}
	if shop.TableName() != "tiktok_sync.shops" {
		t.Errorf("Shop.TableName() = %s, want tiktok_sync.shops", shop.TableName())
	}
}

func TestOAuthToken_TableName(t *testing.T) {
	token := OAuthToken{}
	if token.TableName() != "tiktok_sync.oauth_tokens" {
		t.Errorf("OAuthToken.TableName() = %s, want tiktok_sync.oauth_tokens", token.TableName())
	}
}

func TestProduct_TableName(t *testing.T) {
	p := Product{}
	if p.TableName() != "tiktok_sync.products" {
		t.Errorf("Product.TableName() = %s", p.TableName())
	}
}

func TestSKU_TableName(t *testing.T) {
	s := SKU{}
	if s.TableName() != "tiktok_sync.skus" {
		t.Errorf("SKU.TableName() = %s", s.TableName())
	}
}

func TestOrder_TableName(t *testing.T) {
	order := Order{}
	if order.TableName() != "tiktok_sync.orders" {
		t.Errorf("Order.TableName() = %s", order.TableName())
	}
}

func TestOrderItem_TableName(t *testing.T) {
	item := OrderItem{}
	if item.TableName() != "tiktok_sync.order_items" {
		t.Errorf("OrderItem.TableName() = %s", item.TableName())
	}
}

func TestWebhookEvent_TableName(t *testing.T) {
	event := WebhookEvent{}
	if event.TableName() != "tiktok_sync.webhook_events" {
		t.Errorf("WebhookEvent.TableName() = %s", event.TableName())
	}
}

func TestSyncJob_TableName(t *testing.T) {
	job := SyncJob{}
	if job.TableName() != "tiktok_sync.sync_jobs" {
		t.Errorf("SyncJob.TableName() = %s", job.TableName())
	}
}

func TestOrderStatusConstants(t *testing.T) {
	statuses := []TikTokOrderStatus{
		OrderStatusUnpaid,
		OrderStatusOnHold,
		OrderStatusAwaitingShipment,
		OrderStatusPartiallyShipping,
		OrderStatusAwaitingCollection,
		OrderStatusInTransit,
		OrderStatusDelivered,
		OrderStatusCompleted,
		OrderStatusCancelled,
	}

	expectedValues := []string{
		"UNPAID",
		"ON_HOLD",
		"AWAITING_SHIPMENT",
		"PARTIALLY_SHIPPING",
		"AWAITING_COLLECTION",
		"IN_TRANSIT",
		"DELIVERED",
		"COMPLETED",
		"CANCELLED",
	}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Status %d = %s, want %s", i, status, expectedValues[i])
		}
	}
}

func TestSyncStateConstants(t *testing.T) {
	states := []SyncState{
		SyncStateNew,
		SyncStateSynced,
		SyncStateFailed,
		SyncStateManualReview,
	}

	expectedValues := []string{
		"new",
		"synced",
		"failed",
		"manual_review",
	}

	for i, state := range states {
		if string(state) != expectedValues[i] {
			t.Errorf("State %d = %s, want %s", i, state, expectedValues[i])
		}
	}
}

func TestJobTypeConstants(t *testing.T) {
	types := []JobType{
		JobTypeProcessWebhook,
		JobTypeProductUpsert,
		JobTypeInventoryPush,
		JobTypeShipPackage,
		JobTypeOrderReconcile,
	}

	expectedValues := []string{
		"PROCESS_WEBHOOK_EVENT",
		"PRODUCT_UPSERT",
		"INVENTORY_PUSH",
		"SHIP_PACKAGE",
		"ORDER_RECONCILE",
	}

	for i, jt := range types {
		if string(jt) != expectedValues[i] {
			t.Errorf("JobType %d = %s, want %s", i, jt, expectedValues[i])
		}
	}
}

func TestJobStatusConstants(t *testing.T) {
	statuses := []JobStatus{
		JobStatusQueued,
		JobStatusRunning,
		JobStatusSucceeded,
		JobStatusFailed,
		JobStatusDead,
	}

	expectedValues := []string{
		"queued",
		"running",
		"succeeded",
		"failed",
		"dead",
	}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("JobStatus %d = %s, want %s", i, status, expectedValues[i])
		}
	}
}

func TestEventProcessStatusConstants(t *testing.T) {
	statuses := []EventProcessStatus{
		EventStatusPending,
		EventStatusProcessed,
		EventStatusFailed,
	}

	expectedValues := []string{
		"pending",
		"processed",
		"failed",
	}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("EventProcessStatus %d = %s, want %s", i, status, expectedValues[i])
		}
	}
}

func TestShop_Fields(t *testing.T) {
	now := time.Now()
	shop := Shop{
		ID:        1,
		ShopID:    "shop_123",
		ShopName:  "Test Shop",
		Region:    "VN",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if shop.ID != 1 {
		t.Errorf("Shop.ID = %d", shop.ID)
	}
	if shop.ShopID != "shop_123" {
		t.Errorf("Shop.ShopID = %s", shop.ShopID)
	}
	if shop.ShopName != "Test Shop" {
		t.Errorf("Shop.ShopName = %s", shop.ShopName)
	}
	if shop.Region != "VN" {
		t.Errorf("Shop.Region = %s", shop.Region)
	}
	if shop.Status != "active" {
		t.Errorf("Shop.Status = %s", shop.Status)
	}
}

func TestOrder_Fields(t *testing.T) {
	localOrderID := "local_123"
	order := Order{
		ID:                1,
		TikTokShopID:      "shop_123",
		TikTokOrderID:     "tiktok_order_123",
		TikTokOrderStatus: OrderStatusAwaitingShipment,
		TotalAmount:       100.50,
		Currency:          "VND",
		LocalOrderID:      &localOrderID,
		SyncState:         SyncStateNew,
	}

	if order.TikTokOrderStatus != OrderStatusAwaitingShipment {
		t.Errorf("Order.TikTokOrderStatus = %s", order.TikTokOrderStatus)
	}
	if order.TotalAmount != 100.50 {
		t.Errorf("Order.TotalAmount = %f", order.TotalAmount)
	}
	if *order.LocalOrderID != "local_123" {
		t.Errorf("Order.LocalOrderID = %s", *order.LocalOrderID)
	}
}
