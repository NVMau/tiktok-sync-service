package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGenerateEventID(t *testing.T) {
	shopID := "shop_123"
	eventType := "ORDER_STATUS_CHANGE"
	timestamp := int64(1704067200)
	body := []byte(`{"order_id":"12345"}`)

	eventID1 := generateEventID(shopID, eventType, timestamp, body)
	eventID2 := generateEventID(shopID, eventType, timestamp, body)

	if eventID1 != eventID2 {
		t.Errorf("generateEventID() not deterministic: %s != %s", eventID1, eventID2)
	}

	if len(eventID1) != 32 {
		t.Errorf("generateEventID() length = %d, want 32", len(eventID1))
	}
}

func TestGenerateEventID_DifferentInputs(t *testing.T) {
	body := []byte(`{"order_id":"12345"}`)
	timestamp := int64(1704067200)

	id1 := generateEventID("shop_1", "ORDER_STATUS_CHANGE", timestamp, body)
	id2 := generateEventID("shop_2", "ORDER_STATUS_CHANGE", timestamp, body)

	if id1 == id2 {
		t.Error("Different shop_id should produce different event_id")
	}

	id3 := generateEventID("shop_1", "ORDER_CREATED", timestamp, body)
	if id1 == id3 {
		t.Error("Different event_type should produce different event_id")
	}

	id4 := generateEventID("shop_1", "ORDER_STATUS_CHANGE", timestamp+1, body)
	if id1 == id4 {
		t.Error("Different timestamp should produce different event_id")
	}

	id5 := generateEventID("shop_1", "ORDER_STATUS_CHANGE", timestamp, []byte(`{"order_id":"99999"}`))
	if id1 == id5 {
		t.Error("Different body should produce different event_id")
	}
}

func TestWebhookHandler_ValidateTimestamp(t *testing.T) {
	h := &WebhookHandler{}

	tests := []struct {
		name      string
		timestamp int64
		want      bool
	}{
		{
			name:      "current timestamp",
			timestamp: time.Now().Unix(),
			want:      true,
		},
		{
			name:      "1 minute ago",
			timestamp: time.Now().Unix() - 60,
			want:      true,
		},
		{
			name:      "4 minutes ago",
			timestamp: time.Now().Unix() - 240,
			want:      true,
		},
		{
			name:      "6 minutes ago (expired)",
			timestamp: time.Now().Unix() - 360,
			want:      false,
		},
		{
			name:      "1 hour ago (expired)",
			timestamp: time.Now().Unix() - 3600,
			want:      false,
		},
		{
			name:      "1 minute in future",
			timestamp: time.Now().Unix() + 60,
			want:      true,
		},
		{
			name:      "10 minutes in future (expired)",
			timestamp: time.Now().Unix() + 600,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.validateTimestamp(tt.timestamp)
			if got != tt.want {
				t.Errorf("validateTimestamp(%d) = %v, want %v", tt.timestamp, got, tt.want)
			}
		})
	}
}

func TestWebhookPayload_Parse(t *testing.T) {
	// TikTok webhook uses integer type codes
	jsonData := `{
		"type": 1,
		"shop_id": "7369437808455026474",
		"timestamp": 1704067200,
		"data": {
			"order_id": "1234567890",
			"order_status": "AWAITING_SHIPMENT"
		}
	}`

	var payload WebhookPayload
	err := json.Unmarshal([]byte(jsonData), &payload)

	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}
	if payload.Type != 1 {
		t.Errorf("Type = %d, want 1", payload.Type)
	}
	if payload.ShopID != "7369437808455026474" {
		t.Errorf("ShopID = %s", payload.ShopID)
	}
	if payload.Timestamp != 1704067200 {
		t.Errorf("Timestamp = %d", payload.Timestamp)
	}
	if payload.Data == nil {
		t.Error("Data is nil")
	}
}

func TestGetWebhookTypeName(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{1, "ORDER_STATUS_CHANGE"},
		{2, "REVERSE_STATUS_UPDATE"},
		{3, "RECIPIENT_ADDRESS_UPDATE"},
		{4, "PACKAGE_UPDATE"},
		{999, "UNKNOWN_999"}, // Unknown codes return UNKNOWN_{code}
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := getWebhookTypeName(tt.code)
			if got != tt.want {
				t.Errorf("getWebhookTypeName(%d) = %s, want %s", tt.code, got, tt.want)
			}
		})
	}
}

func BenchmarkGenerateEventID(b *testing.B) {
	shopID := "shop_123"
	eventType := "ORDER_STATUS_CHANGE"
	timestamp := int64(1704067200)
	body := []byte(`{"order_id":"12345","order_status":"AWAITING_SHIPMENT","update_time":1704067200}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generateEventID(shopID, eventType, timestamp, body)
	}
}
