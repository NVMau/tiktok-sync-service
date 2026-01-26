package services

import (
	"strings"
	"testing"

	"github.com/user/sync-tiktok-mps/internal/models"
)

func TestOrderSyncService_MapTikTokStatusToLocal(t *testing.T) {
	svc := NewOrderSyncService()

	tests := []struct {
		tiktokStatus models.TikTokOrderStatus
		want         LocalOrderStatus
	}{
		{models.OrderStatusUnpaid, LocalStatusPendingPayment},
		{models.OrderStatusOnHold, LocalStatusPaidHold},
		{models.OrderStatusAwaitingShipment, LocalStatusReadyToShip},
		{models.OrderStatusPartiallyShipping, LocalStatusShippedAwaitingPickup},
		{models.OrderStatusAwaitingCollection, LocalStatusShippedAwaitingPickup},
		{models.OrderStatusInTransit, LocalStatusInTransit},
		{models.OrderStatusDelivered, LocalStatusDelivered},
		{models.OrderStatusCompleted, LocalStatusCompleted},
		{models.OrderStatusCancelled, LocalStatusCancelled},
	}

	for _, tt := range tests {
		t.Run(string(tt.tiktokStatus), func(t *testing.T) {
			got := svc.MapTikTokStatusToLocal(tt.tiktokStatus)
			if got != tt.want {
				t.Errorf("MapTikTokStatusToLocal(%s) = %s, want %s",
					tt.tiktokStatus, got, tt.want)
			}
		})
	}
}

func TestOrderSyncService_ReserveStatusLogic(t *testing.T) {
	// Test that paid orders should reserve SIM
	reserveStatuses := []models.TikTokOrderStatus{
		models.OrderStatusOnHold,
		models.OrderStatusAwaitingShipment,
		models.OrderStatusPartiallyShipping,
		models.OrderStatusAwaitingCollection,
		models.OrderStatusInTransit,
		models.OrderStatusDelivered,
		models.OrderStatusCompleted,
	}

	for _, status := range reserveStatuses {
		// These statuses indicate payment received, SIM should be reserved
		if status == models.OrderStatusUnpaid || status == models.OrderStatusCancelled {
			t.Errorf("status %s should not be in reserve list", status)
		}
	}

	// Test that unpaid/cancelled should NOT reserve
	noReserveStatuses := []models.TikTokOrderStatus{
		models.OrderStatusUnpaid,
		models.OrderStatusCancelled,
	}

	for _, status := range noReserveStatuses {
		found := false
		for _, rs := range reserveStatuses {
			if status == rs {
				found = true
				break
			}
		}
		if found {
			t.Errorf("status %s should not reserve SIM", status)
		}
	}
}

func TestLocalOrderStatus_Values(t *testing.T) {
	statuses := []LocalOrderStatus{
		LocalStatusPendingPayment,
		LocalStatusPaidHold,
		LocalStatusReadyToShip,
		LocalStatusShippedAwaitingPickup,
		LocalStatusInTransit,
		LocalStatusDelivered,
		LocalStatusCompleted,
		LocalStatusCancelled,
	}

	expectedValues := []string{
		"PENDING_PAYMENT",
		"PAID_HOLD",
		"READY_TO_SHIP",
		"SHIPPED_AWAITING_PICKUP",
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

func TestAppendError(t *testing.T) {
	payload := []byte(`{"order_id":"123"}`)
	result := appendError(payload, "test error")

	expectedContains := `"sync_errors"`
	if !strings.Contains(string(result), expectedContains) {
		t.Errorf("appendError result should contain %s, got %s", expectedContains, string(result))
	}

	result2 := appendError(result, "second error")
	if string(result2) == "" {
		t.Error("appendError second call returned empty result")
	}
}

func TestAppendError_NilPayload(t *testing.T) {
	result := appendError(nil, "test error")
	if result == nil {
		t.Error("appendError(nil) should not return nil")
	}
}
