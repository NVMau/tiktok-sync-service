package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type FulfillmentAPI struct {
	client       *Client
	tokenManager *TokenManager
}

func NewFulfillmentAPI(client *Client, tokenManager *TokenManager) *FulfillmentAPI {
	return &FulfillmentAPI{
		client:       client,
		tokenManager: tokenManager,
	}
}

type ShipPackageRequest struct {
	OrderID            string `json:"order_id"`
	PackageID          string `json:"package_id,omitempty"`
	TrackingNumber     string `json:"tracking_number"`
	ShippingProviderID string `json:"shipping_provider_id"`
	PickUpType         int    `json:"pick_up_type,omitempty"`
	SelfShipment       *SelfShipmentInfo `json:"self_shipment,omitempty"`
}

type SelfShipmentInfo struct {
	TrackingNumber     string `json:"tracking_number"`
	ShippingProviderID string `json:"shipping_provider_id"`
}

type ShipPackageResponse struct {
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	FailedOrderIDs []string `json:"failed_order_ids"`
}

func (f *FulfillmentAPI) ShipPackage(ctx context.Context, shopID uint, shopCipher string, req *ShipPackageRequest) (*ShipPackageResponse, error) {
	accessToken, err := f.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Request(ctx, http.MethodPost, "/fulfillment/202309/packages/ship", nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        req,
	})
	if err != nil {
		return nil, err
	}

	var result ShipPackageResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ship response: %w", err)
	}

	return &result, nil
}

type GetPackageDetailRequest struct {
	PackageID string
}

type PackageDetail struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	TrackingNumber   string `json:"tracking_number"`
	ShippingProvider struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"shipping_provider"`
	OrderID      string   `json:"order_id"`
	OrderLineItemIDs []string `json:"order_line_item_ids"`
	CreateTime   int64  `json:"create_time"`
	UpdateTime   int64  `json:"update_time"`
	DeliveryType string `json:"delivery_type"`
}

func (f *FulfillmentAPI) GetPackageDetail(ctx context.Context, shopID uint, shopCipher, packageID string) (*PackageDetail, error) {
	accessToken, err := f.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return nil, err
	}

	params := map[string]string{}

	resp, err := f.client.Request(ctx, http.MethodGet, fmt.Sprintf("/fulfillment/202309/packages/%s", packageID), params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
	})
	if err != nil {
		return nil, err
	}

	var result PackageDetail
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse package detail: %w", err)
	}

	return &result, nil
}

type MarkPackageDeliveredRequest struct {
	PackageID string `json:"package_id"`
}

func (f *FulfillmentAPI) MarkPackageDelivered(ctx context.Context, shopID uint, shopCipher string, req *MarkPackageDeliveredRequest) error {
	accessToken, err := f.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return err
	}

	_, err = f.client.Request(ctx, http.MethodPost, "/fulfillment/202309/packages/deliver", nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        req,
	})

	return err
}
