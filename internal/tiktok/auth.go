package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/models"
)

type AuthClient struct {
	httpClient *http.Client
	cfg        *config.Config
}

func NewAuthClient(cfg *config.Config) *AuthClient {
	return &AuthClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cfg: cfg,
	}
}

type TokenResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      struct {
		AccessToken          string   `json:"access_token"`
		AccessTokenExpireIn  int64    `json:"access_token_expire_in"`
		RefreshToken         string   `json:"refresh_token"`
		RefreshTokenExpireIn int64    `json:"refresh_token_expire_in"`
		OpenID               string   `json:"open_id"`
		SellerName           string   `json:"seller_name"`
		SellerBaseRegion     string   `json:"seller_base_region"`
		UserType             int      `json:"user_type"`
		GrantedScopes        []string `json:"granted_scopes"`
	} `json:"data"`
}

func (a *AuthClient) GetAccessToken(ctx context.Context, authCode string) (*TokenResponse, error) {
	params := url.Values{}
	params.Set("app_key", a.cfg.TikTokAppKey)
	params.Set("app_secret", a.cfg.TikTokAppSecret)
	params.Set("auth_code", authCode)
	params.Set("grant_type", "authorized_code")

	reqURL := fmt.Sprintf("%s/api/v2/token/get?%s", a.cfg.TikTokAuthBaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.Code != 0 {
		return nil, fmt.Errorf("auth error: code=%d, message=%s", tokenResp.Code, tokenResp.Message)
	}

	return &tokenResp, nil
}

func (a *AuthClient) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	params := url.Values{}
	params.Set("app_key", a.cfg.TikTokAppKey)
	params.Set("app_secret", a.cfg.TikTokAppSecret)
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")

	reqURL := fmt.Sprintf("%s/api/v2/token/refresh?%s", a.cfg.TikTokAuthBaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.Code != 0 {
		return nil, fmt.Errorf("refresh error: code=%d, message=%s", tokenResp.Code, tokenResp.Message)
	}

	return &tokenResp, nil
}

type TokenManager struct {
	authClient *AuthClient
}

func NewTokenManager(cfg *config.Config) *TokenManager {
	return &TokenManager{
		authClient: NewAuthClient(cfg),
	}
}

func (tm *TokenManager) GetValidToken(ctx context.Context, tiktokShopID string) (string, error) {
	var token models.OAuthToken
	if err := database.DB.Where("tik_tok_shop_id = ?", tiktokShopID).First(&token).Error; err != nil {
		return "", fmt.Errorf("token not found for shop %s: %w", tiktokShopID, err)
	}

	if time.Now().Before(token.ExpiresAt.Add(-5 * time.Minute)) {
		return token.AccessToken, nil
	}

	tokenResp, err := tm.authClient.RefreshAccessToken(ctx, token.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	token.AccessToken = tokenResp.Data.AccessToken
	token.RefreshToken = tokenResp.Data.RefreshToken
	token.ExpiresAt = time.Unix(tokenResp.Data.AccessTokenExpireIn, 0)
	token.UpdatedAt = time.Now()

	if err := database.DB.Save(&token).Error; err != nil {
		return "", fmt.Errorf("failed to save refreshed token: %w", err)
	}

	return token.AccessToken, nil
}

func (tm *TokenManager) SaveToken(ctx context.Context, tiktokShopID string, tokenResp *TokenResponse) error {
	scopesJSON, _ := json.Marshal(tokenResp.Data.GrantedScopes)

	token := models.OAuthToken{
		TikTokShopID: tiktokShopID,
		AccessToken:  tokenResp.Data.AccessToken,
		RefreshToken: tokenResp.Data.RefreshToken,
		ExpiresAt:    time.Unix(tokenResp.Data.AccessTokenExpireIn, 0),
		Scopes:       string(scopesJSON),
		UpdatedAt:    time.Now(),
	}

	result := database.DB.Where("tik_tok_shop_id = ?", tiktokShopID).
		Assign(token).
		FirstOrCreate(&token)

	if result.Error != nil {
		return fmt.Errorf("failed to save token: %w", result.Error)
	}

	return nil
}
