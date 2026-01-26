package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ShopsAPI struct {
	client       *Client
	tokenManager *TokenManager
}

func NewShopsAPI(client *Client, tokenManager *TokenManager) *ShopsAPI {
	return &ShopsAPI{
		client:       client,
		tokenManager: tokenManager,
	}
}

type AuthorizedShop struct {
	ID         string   `json:"id"`
	Cipher     string   `json:"cipher"`
	Code       string   `json:"code"`
	Name       string   `json:"name"`
	Region     string   `json:"region"`
	SellerType string   `json:"seller_type"`
}

type GetAuthorizedShopsResponse struct {
	Shops []AuthorizedShop `json:"shops"`
}

func (s *ShopsAPI) GetAuthorizedShops(ctx context.Context, accessToken string) ([]AuthorizedShop, error) {
	params := map[string]string{}

	resp, err := s.client.Request(ctx, http.MethodGet, "/authorization/202309/shops", params, &RequestOption{
		AccessToken: accessToken,
	})
	if err != nil {
		return nil, err
	}

	var result GetAuthorizedShopsResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse shops: %w", err)
	}

	return result.Shops, nil
}
