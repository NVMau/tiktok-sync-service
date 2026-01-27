package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type LogisticsAPI struct {
	client       *Client
	tokenManager *TokenManager
}

func NewLogisticsAPI(client *Client, tokenManager *TokenManager) *LogisticsAPI {
	return &LogisticsAPI{
		client:       client,
		tokenManager: tokenManager,
	}
}

type Warehouse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	SubType        string `json:"sub_type"`
	EffectStatus   string `json:"effect_status"`
	IsDefault      bool   `json:"is_default"`
	WarehouseAddress struct {
		RegionCode    string   `json:"region_code"`
		State         string   `json:"state"`
		City          string   `json:"city"`
		District      string   `json:"district"`
		Town          string   `json:"town"`
		FullAddress   string   `json:"full_address"`
		AddressLineList []string `json:"address_line_list"`
		PostalCode    string   `json:"postal_code"`
	} `json:"warehouse_address"`
}

type GetWarehousesResponse struct {
	Warehouses []Warehouse `json:"warehouses"`
}

func (l *LogisticsAPI) GetWarehouses(ctx context.Context, tiktokShopID string, shopCipher string) ([]Warehouse, error) {
	accessToken, err := l.tokenManager.GetValidToken(ctx, tiktokShopID)
	if err != nil {
		return nil, err
	}

	params := map[string]string{}

	resp, err := l.client.Request(ctx, http.MethodGet, "/logistics/202309/warehouses", params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
	})
	if err != nil {
		return nil, err
	}

	var result GetWarehousesResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse warehouses: %w", err)
	}

	return result.Warehouses, nil
}

type ShippingProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetShippingProvidersResponse struct {
	ShippingProviders []ShippingProvider `json:"shipping_providers"`
}

func (l *LogisticsAPI) GetShippingProviders(ctx context.Context, tiktokShopID string, shopCipher string) ([]ShippingProvider, error) {
	accessToken, err := l.tokenManager.GetValidToken(ctx, tiktokShopID)
	if err != nil {
		return nil, err
	}

	params := map[string]string{}

	resp, err := l.client.Request(ctx, http.MethodGet, "/logistics/202309/shipping_providers", params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
	})
	if err != nil {
		return nil, err
	}

	var result GetShippingProvidersResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse shipping providers: %w", err)
	}

	return result.ShippingProviders, nil
}

type DeliveryOption struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ShippingType string `json:"shipping_type"`
}

type GetDeliveryOptionsResponse struct {
	DeliveryOptions []DeliveryOption `json:"delivery_options"`
}

func (l *LogisticsAPI) GetDeliveryOptions(ctx context.Context, tiktokShopID string, shopCipher string) ([]DeliveryOption, error) {
	accessToken, err := l.tokenManager.GetValidToken(ctx, tiktokShopID)
	if err != nil {
		return nil, err
	}

	params := map[string]string{}

	resp, err := l.client.Request(ctx, http.MethodGet, "/logistics/202309/delivery_options", params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
	})
	if err != nil {
		return nil, err
	}

	var result GetDeliveryOptionsResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse delivery options: %w", err)
	}

	return result.DeliveryOptions, nil
}
