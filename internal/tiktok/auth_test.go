package tiktok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/sync-tiktok-mps/internal/config"
)

func TestNewAuthClient(t *testing.T) {
	cfg := &config.Config{
		TikTokAppKey:      "test_key",
		TikTokAppSecret:   "test_secret",
		TikTokAuthBaseURL: "https://auth.test.com",
	}

	client := NewAuthClient(cfg)

	if client == nil {
		t.Fatal("NewAuthClient() returned nil")
	}
	if client.cfg != cfg {
		t.Error("NewAuthClient() config mismatch")
	}
}

func TestAuthClient_GetAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/token/get" {
			t.Errorf("Wrong path: %s", r.URL.Path)
		}

		appKey := r.URL.Query().Get("app_key")
		appSecret := r.URL.Query().Get("app_secret")
		authCode := r.URL.Query().Get("auth_code")
		grantType := r.URL.Query().Get("grant_type")

		if appKey != "test_key" {
			t.Errorf("Wrong app_key: %s", appKey)
		}
		if appSecret != "test_secret" {
			t.Errorf("Wrong app_secret: %s", appSecret)
		}
		if authCode != "test_auth_code" {
			t.Errorf("Wrong auth_code: %s", authCode)
		}
		if grantType != "authorized_code" {
			t.Errorf("Wrong grant_type: %s", grantType)
		}

		resp := TokenResponse{
			Code:    0,
			Message: "success",
		}
		resp.Data.AccessToken = "access_token_123"
		resp.Data.RefreshToken = "refresh_token_456"
		resp.Data.AccessTokenExpireIn = 1700000000
		resp.Data.RefreshTokenExpireIn = 1730000000
		resp.Data.SellerName = "Test Seller"
		resp.Data.SellerBaseRegion = "VN"
		resp.Data.GrantedScopes = []string{"order.read", "product.write"}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:      "test_key",
		TikTokAppSecret:   "test_secret",
		TikTokAuthBaseURL: server.URL,
	}

	client := NewAuthClient(cfg)
	resp, err := client.GetAccessToken(context.Background(), "test_auth_code")

	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if resp.Data.AccessToken != "access_token_123" {
		t.Errorf("GetAccessToken() access_token = %s", resp.Data.AccessToken)
	}
	if resp.Data.SellerName != "Test Seller" {
		t.Errorf("GetAccessToken() seller_name = %s", resp.Data.SellerName)
	}
	if len(resp.Data.GrantedScopes) != 2 {
		t.Errorf("GetAccessToken() scopes count = %d", len(resp.Data.GrantedScopes))
	}
}

func TestAuthClient_GetAccessToken_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			Code:    105001,
			Message: "Invalid auth_code",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:      "test_key",
		TikTokAppSecret:   "test_secret",
		TikTokAuthBaseURL: server.URL,
	}

	client := NewAuthClient(cfg)
	_, err := client.GetAccessToken(context.Background(), "invalid_code")

	if err == nil {
		t.Fatal("GetAccessToken() should return error for non-zero code")
	}
}

func TestAuthClient_RefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/token/refresh" {
			t.Errorf("Wrong path: %s", r.URL.Path)
		}

		refreshToken := r.URL.Query().Get("refresh_token")
		grantType := r.URL.Query().Get("grant_type")

		if refreshToken != "old_refresh_token" {
			t.Errorf("Wrong refresh_token: %s", refreshToken)
		}
		if grantType != "refresh_token" {
			t.Errorf("Wrong grant_type: %s", grantType)
		}

		resp := TokenResponse{
			Code:    0,
			Message: "success",
		}
		resp.Data.AccessToken = "new_access_token"
		resp.Data.RefreshToken = "new_refresh_token"
		resp.Data.AccessTokenExpireIn = 1700000000

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		TikTokAppKey:      "test_key",
		TikTokAppSecret:   "test_secret",
		TikTokAuthBaseURL: server.URL,
	}

	client := NewAuthClient(cfg)
	resp, err := client.RefreshAccessToken(context.Background(), "old_refresh_token")

	if err != nil {
		t.Fatalf("RefreshAccessToken() error = %v", err)
	}
	if resp.Data.AccessToken != "new_access_token" {
		t.Errorf("RefreshAccessToken() access_token = %s", resp.Data.AccessToken)
	}
	if resp.Data.RefreshToken != "new_refresh_token" {
		t.Errorf("RefreshAccessToken() refresh_token = %s", resp.Data.RefreshToken)
	}
}

func TestNewTokenManager(t *testing.T) {
	cfg := &config.Config{
		TikTokAppKey:      "test_key",
		TikTokAppSecret:   "test_secret",
		TikTokAuthBaseURL: "https://auth.test.com",
	}

	tm := NewTokenManager(cfg)

	if tm == nil {
		t.Fatal("NewTokenManager() returned nil")
	}
	if tm.authClient == nil {
		t.Error("NewTokenManager() authClient is nil")
	}
}
