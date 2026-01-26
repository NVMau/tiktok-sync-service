package tiktok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/user/sync-tiktok-mps/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		TikTokAppKey:     "test_app_key",
		TikTokAppSecret:  "test_app_secret",
		TikTokAPIBaseURL: "https://api.test.com",
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.cfg != cfg {
		t.Error("NewClient() config mismatch")
	}
	if client.rateLimiter == nil {
		t.Error("NewClient() rateLimiter is nil")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(10)

	start := time.Now()
	rl.Wait()
	rl.Wait()
	elapsed := time.Since(start)

	if elapsed < 90*time.Millisecond {
		t.Errorf("RateLimiter should wait ~100ms between requests, got %v", elapsed)
	}
}

func TestClient_buildURL(t *testing.T) {
	cfg := &config.Config{
		TikTokAPIBaseURL: "https://api.test.com",
	}
	client := NewClient(cfg)

	tests := []struct {
		name   string
		path   string
		params map[string]string
		want   string
	}{
		{
			name:   "simple path",
			path:   "/test",
			params: map[string]string{},
			want:   "https://api.test.com/test",
		},
		{
			name: "path with params",
			path: "/orders",
			params: map[string]string{
				"app_key": "key123",
				"sign":    "sig456",
			},
			want: "https://api.test.com/orders?app_key=key123&sign=sig456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.buildURL(tt.path, tt.params)
			if got != tt.want {
				t.Errorf("buildURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_Request_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:      0,
			Message:   "success",
			RequestID: "req_123",
			Data:      json.RawMessage(`{"test":"data"}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:     "test_key",
		TikTokAppSecret:  "test_secret",
		TikTokAPIBaseURL: server.URL,
	}
	client := NewClient(cfg)

	resp, err := client.Request(context.Background(), http.MethodGet, "/test", nil, nil)

	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("Request() code = %d, want 0", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Request() message = %s, want success", resp.Message)
	}
}

func TestClient_Request_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := APIResponse{
			Code:      105001,
			Message:   "Invalid sign",
			RequestID: "req_123",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:     "test_key",
		TikTokAppSecret:  "test_secret",
		TikTokAPIBaseURL: server.URL,
	}
	client := NewClient(cfg)

	resp, err := client.Request(context.Background(), http.MethodGet, "/test", nil, nil)

	if err == nil {
		t.Fatal("Request() should return error for non-zero code")
	}
	if resp == nil {
		t.Fatal("Request() should still return response on API error")
	}
	if resp.Code != 105001 {
		t.Errorf("Request() code = %d, want 105001", resp.Code)
	}
}

func TestClient_Request_WithBody(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := APIResponse{
			Code:    0,
			Message: "success",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:     "test_key",
		TikTokAppSecret:  "test_secret",
		TikTokAPIBaseURL: server.URL,
	}
	client := NewClient(cfg)

	body := map[string]string{"order_id": "12345"}
	_, err := client.Request(context.Background(), http.MethodPost, "/test", nil, &RequestOption{
		Body: body,
	})

	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if receivedBody["order_id"] != "12345" {
		t.Errorf("Request() body not sent correctly")
	}
}

func TestClient_Request_WithAccessToken(t *testing.T) {
	var receivedToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("x-tts-access-token")
		resp := APIResponse{Code: 0, Message: "success"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:     "test_key",
		TikTokAppSecret:  "test_secret",
		TikTokAPIBaseURL: server.URL,
	}
	client := NewClient(cfg)

	_, err := client.Request(context.Background(), http.MethodGet, "/test", nil, &RequestOption{
		AccessToken: "my_access_token",
	})

	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if receivedToken != "my_access_token" {
		t.Errorf("Request() access token not set in header: got %s", receivedToken)
	}
}

func TestClient_doWithRetry_ServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := APIResponse{Code: 0, Message: "success"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:     "test_key",
		TikTokAppSecret:  "test_secret",
		TikTokAPIBaseURL: server.URL,
	}
	client := NewClient(cfg)

	_, err := client.Request(context.Background(), http.MethodGet, "/test", nil, nil)

	if err != nil {
		t.Fatalf("Request() should succeed after retries, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}
