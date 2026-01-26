package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OrdersAPI struct {
	client       *Client
	tokenManager *TokenManager
}

func NewOrdersAPI(client *Client, tokenManager *TokenManager) *OrdersAPI {
	return &OrdersAPI{
		client:       client,
		tokenManager: tokenManager,
	}
}

type OrderListRequest struct {
	ShopID       uint
	ShopCipher   string
	PageSize     int
	PageToken    string
	SortOrder    string
	SortField    string
	CreateTimeGe int64
	CreateTimeLt int64
	UpdateTimeGe int64
	UpdateTimeLt int64
	OrderStatus  string
}

type OrderListResponse struct {
	Orders        []OrderSummary `json:"orders"`
	NextPageToken string         `json:"next_page_token"`
	TotalCount    int            `json:"total_count"`
}

type OrderSummary struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	PaymentStatus   string `json:"payment_method_name"`
	FulfillmentType string `json:"fulfillment_type"`
	CreateTime      int64  `json:"create_time"`
	UpdateTime      int64  `json:"update_time"`
}

func (o *OrdersAPI) GetOrderList(ctx context.Context, req *OrderListRequest) (*OrderListResponse, error) {
	accessToken, err := o.tokenManager.GetValidToken(ctx, req.ShopID)
	if err != nil {
		return nil, err
	}

	// Query parameters
	params := map[string]string{
		"page_size": fmt.Sprintf("%d", req.PageSize),
	}
	if req.PageToken != "" {
		params["page_token"] = req.PageToken
	}
	if req.SortOrder != "" {
		params["sort_order"] = req.SortOrder
	}
	if req.SortField != "" {
		params["sort_field"] = req.SortField
	}

	// Body parameters (filters)
	body := make(map[string]interface{})
	if req.CreateTimeGe > 0 {
		body["create_time_ge"] = req.CreateTimeGe
	}
	if req.CreateTimeLt > 0 {
		body["create_time_lt"] = req.CreateTimeLt
	}
	if req.UpdateTimeGe > 0 {
		body["update_time_ge"] = req.UpdateTimeGe
	}
	if req.UpdateTimeLt > 0 {
		body["update_time_lt"] = req.UpdateTimeLt
	}
	if req.OrderStatus != "" {
		body["order_status"] = req.OrderStatus
	}

	var bodyPtr interface{}
	if len(body) > 0 {
		bodyPtr = body
	}

	resp, err := o.client.Request(ctx, http.MethodPost, "/order/202309/orders/search", params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  req.ShopCipher,
		Body:        bodyPtr,
	})
	if err != nil {
		return nil, err
	}

	var result OrderListResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order list: %w", err)
	}

	return &result, nil
}

type OrderDetailResponse struct {
	ID                string           `json:"id"`
	Status            string           `json:"status"`
	PaymentMethodName string           `json:"payment_method_name"`
	FulfillmentType   string           `json:"fulfillment_type"`
	BuyerMessage      string           `json:"buyer_message"`
	SellerNote        string           `json:"seller_note"`
	CreateTime        int64            `json:"create_time"`
	UpdateTime        int64            `json:"update_time"`
	PaidTime          int64            `json:"paid_time"`
	RtsSLA            int64            `json:"rts_sla_time"`
	TtsSLA            int64            `json:"tts_sla_time"`
	CancelOrderSLA    int64            `json:"cancel_order_sla_time"`
	DeliveryDueSLA    int64            `json:"delivery_due_sla_time"`
	Payment           OrderPayment     `json:"payment"`
	RecipientAddress  RecipientAddress `json:"recipient_address"`
	LineItems         []OrderLineItem  `json:"line_items"`
	Packages          []OrderPackage   `json:"packages"`
}

type OrderPayment struct {
	Currency                   string `json:"currency"`
	SubTotal                   string `json:"sub_total"`
	ShippingFee                string `json:"shipping_fee"`
	SellerDiscount             string `json:"seller_discount"`
	PlatformDiscount           string `json:"platform_discount"`
	TotalAmount                string `json:"total_amount"`
	OriginalTotalProductPrice  string `json:"original_total_product_price"`
	OriginalShippingFee        string `json:"original_shipping_fee"`
	ShippingFeeSellerDiscount  string `json:"shipping_fee_seller_discount"`
	ShippingFeePlatformDiscount string `json:"shipping_fee_platform_discount"`
	Tax                        string `json:"tax"`
	ProductTax                 string `json:"product_tax"`
	ShippingFeeTax             string `json:"shipping_fee_tax"`
}

type RecipientAddress struct {
	FullAddress    string   `json:"full_address"`
	RegionCode     string   `json:"region_code"`
	PhoneNumber    string   `json:"phone_number"`
	Name           string   `json:"name"`
	AddressDetail  string   `json:"address_detail"`
	AddressLineList []string `json:"address_line_list"`
	PostalCode     string   `json:"postal_code"`
	DistrictInfo   []struct {
		AddressLevel     string `json:"address_level"`
		AddressLevelName string `json:"address_level_name"`
		AddressName      string `json:"address_name"`
	} `json:"district_info"`
}

type OrderLineItem struct {
	ID              string `json:"id"`
	ProductID       string `json:"product_id"`
	ProductName     string `json:"product_name"`
	SkuID           string `json:"sku_id"`
	SkuName         string `json:"sku_name"`
	SellerSku       string `json:"seller_sku"`
	SkuImage        string `json:"sku_image"`
	Quantity        int    `json:"quantity"`
	OriginalPrice   string `json:"original_price"`
	SalePrice       string `json:"sale_price"`
	PlatformDiscount string `json:"platform_discount"`
	SellerDiscount  string `json:"seller_discount"`
	SkuType         string `json:"sku_type"`
	IsGift          bool   `json:"is_gift"`
	ItemTax         []struct {
		TaxType   string `json:"tax_type"`
		TaxAmount string `json:"tax_amount"`
		TaxRate   string `json:"tax_rate"`
	} `json:"item_tax"`
}

type OrderPackage struct {
	ID             string `json:"id"`
	TrackingNumber string `json:"tracking_number"`
	ProviderID     string `json:"shipping_provider_id"`
	ProviderName   string `json:"shipping_provider_name"`
}

type OrderDetailAPIResponse struct {
	Orders []OrderDetailResponse `json:"orders"`
}

func (o *OrdersAPI) GetOrderDetail(ctx context.Context, shopID uint, shopCipher, orderID string) (*OrderDetailResponse, error) {
	accessToken, err := o.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return nil, err
	}

	// API uses ids as query param, not path param
	params := map[string]string{
		"ids": orderID,
	}

	resp, err := o.client.Request(ctx, http.MethodGet, "/order/202309/orders", params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
	})
	if err != nil {
		return nil, err
	}

	var result OrderDetailAPIResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse order detail: %w", err)
	}

	if len(result.Orders) == 0 {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return &result.Orders[0], nil
}

func (o *OrdersAPI) GetRecentOrders(ctx context.Context, shopID uint, shopCipher string, since time.Duration) ([]OrderSummary, error) {
	now := time.Now()
	createTimeGe := now.Add(-since).Unix()

	var allOrders []OrderSummary
	pageToken := ""

	for {
		resp, err := o.GetOrderList(ctx, &OrderListRequest{
			ShopID:       shopID,
			ShopCipher:   shopCipher,
			PageSize:     50,
			PageToken:    pageToken,
			SortOrder:    "DESC",
			SortField:    "create_time",
			CreateTimeGe: createTimeGe,
		})
		if err != nil {
			return nil, err
		}

		allOrders = append(allOrders, resp.Orders...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return allOrders, nil
}
