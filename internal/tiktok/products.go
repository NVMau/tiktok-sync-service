package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/logger"
)

type ProductsAPI struct {
	client       *Client
	tokenManager *TokenManager
}

func NewProductsAPI(client *Client, tokenManager *TokenManager) *ProductsAPI {
	return &ProductsAPI{
		client:       client,
		tokenManager: tokenManager,
	}
}

type ProductListRequest struct {
	ShopID     uint
	ShopCipher string
	PageSize   int
	PageToken  string
	Status     string
}

type ProductListResponse struct {
	Products      []ProductSummary `json:"products"`
	NextPageToken string           `json:"next_page_token"`
	TotalCount    int              `json:"total_count"`
}

type ProductSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
}

func (p *ProductsAPI) GetProductList(ctx context.Context, req *ProductListRequest) (*ProductListResponse, error) {
	accessToken, err := p.tokenManager.GetValidToken(ctx, req.ShopID)
	if err != nil {
		return nil, err
	}

	// page_size and page_token are query parameters, not body
	params := map[string]string{
		"page_size": fmt.Sprintf("%d", req.PageSize),
	}
	if req.PageToken != "" {
		params["page_token"] = req.PageToken
	}

	// Body only contains filter fields
	var body map[string]interface{}
	if req.Status != "" {
		body = map[string]interface{}{
			"status": req.Status,
		}
	}

	resp, err := p.client.Request(ctx, http.MethodPost, "/product/202309/products/search", params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  req.ShopCipher,
		Body:        body,
	})
	if err != nil {
		return nil, err
	}

	var result ProductListResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse product list: %w", err)
	}

	return &result, nil
}

type ProductDetail struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	CategoryID   string        `json:"category_id"`
	Status       string        `json:"status"`
	Brand        *ProductBrand `json:"brand"`
	MainImages   []ProductImage `json:"main_images"`
	SKUs         []ProductSKU  `json:"skus"`
	CreateTime   int64         `json:"create_time"`
	UpdateTime   int64         `json:"update_time"`
	IsCodAllowed bool          `json:"is_cod_allowed"`
	PackageDimensions struct {
		Length string `json:"length"`
		Width  string `json:"width"`
		Height string `json:"height"`
		Unit   string `json:"unit"`
	} `json:"package_dimensions"`
	PackageWeight struct {
		Value string `json:"value"`
		Unit  string `json:"unit"`
	} `json:"package_weight"`
}

type ProductBrand struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProductImage struct {
	URI    string `json:"uri"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type ProductSKU struct {
	ID          string `json:"id"`
	SellerSku   string `json:"seller_sku"`
	Price       ProductPrice `json:"price"`
	Inventory   []ProductInventory `json:"inventory"`
	SalesAttributes []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		ValueID   string `json:"value_id"`
		ValueName string `json:"value_name"`
	} `json:"sales_attributes"`
}

type ProductPrice struct {
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	SalePrice    string `json:"sale_price"`
	OriginalPrice string `json:"original_price"`
}

type ProductInventory struct {
	WarehouseID string `json:"warehouse_id"`
	Quantity    int    `json:"quantity"`
}

func (p *ProductsAPI) GetProductDetail(ctx context.Context, shopID uint, shopCipher, productID string) (*ProductDetail, error) {
	accessToken, err := p.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return nil, err
	}

	params := map[string]string{}

	resp, err := p.client.Request(ctx, http.MethodGet, fmt.Sprintf("/product/202309/products/%s", productID), params, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
	})
	if err != nil {
		return nil, err
	}

	var result ProductDetail
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse product detail: %w", err)
	}

	return &result, nil
}

type UpdateInventoryRequest struct {
	ProductID   string `json:"product_id"`
	SKUs        []UpdateSKUInventory `json:"skus"`
}

type UpdateSKUInventory struct {
	ID          string `json:"id"`
	Inventory   []InventoryUpdate `json:"inventory"`
}

type InventoryUpdate struct {
	WarehouseID string `json:"warehouse_id"`
	Quantity    int    `json:"quantity"`
}

func (p *ProductsAPI) UpdateInventory(ctx context.Context, shopID uint, shopCipher string, req *UpdateInventoryRequest) error {
	accessToken, err := p.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return err
	}

	_, err = p.client.Request(ctx, http.MethodPost, fmt.Sprintf("/product/202309/products/%s/inventory/update", req.ProductID), nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        req,
	})

	return err
}

type UpdatePriceRequest struct {
	ProductID string `json:"product_id"`
	SKUs      []UpdateSKUPrice `json:"skus"`
}

type UpdateSKUPrice struct {
	ID          string `json:"id"`
	OriginalPrice string `json:"original_price,omitempty"`
	SalePrice   string `json:"sale_price,omitempty"`
}

func (p *ProductsAPI) UpdatePrice(ctx context.Context, shopID uint, shopCipher string, req *UpdatePriceRequest) error {
	accessToken, err := p.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return err
	}

	_, err = p.client.Request(ctx, http.MethodPost, fmt.Sprintf("/product/202309/products/%s/prices/update", req.ProductID), nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        req,
	})

	return err
}

type ActivateProductRequest struct {
	ProductIDs []string `json:"product_ids"`
}

func (p *ProductsAPI) ActivateProducts(ctx context.Context, shopID uint, shopCipher string, productIDs []string) error {
	accessToken, err := p.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return err
	}

	_, err = p.client.Request(ctx, http.MethodPost, "/product/202309/products/activate", nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        &ActivateProductRequest{ProductIDs: productIDs},
	})

	return err
}

type DeactivateProductRequest struct {
	ProductIDs []string `json:"product_ids"`
}

func (p *ProductsAPI) DeactivateProducts(ctx context.Context, shopID uint, shopCipher string, productIDs []string) error {
	accessToken, err := p.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return err
	}

	_, err = p.client.Request(ctx, http.MethodPost, "/product/202309/products/deactivate", nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        &DeactivateProductRequest{ProductIDs: productIDs},
	})

	return err
}

// =============================================================================
// PARTIAL EDIT PRODUCT - Add/Edit SKU
// =============================================================================

type PartialEditProductRequest struct {
	SKUs []PartialEditSKU `json:"skus"`
}

type PartialEditSKU struct {
	ID              string                  `json:"id,omitempty"`                    // With ID = edit SKU, without = create new
	SellerSKU       string                  `json:"seller_sku"`                      // Phone number
	SalesAttributes []PartialEditAttribute  `json:"sales_attributes,omitempty"`      // Omit if nil/empty
	Price           PartialEditPrice        `json:"price"`
	Inventory       []PartialEditInventory  `json:"inventory"`
}

type PartialEditAttribute struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	ValueID   string `json:"value_id,omitempty"`
	ValueName string `json:"value_name,omitempty"`
}

type PartialEditPrice struct {
	Amount    string `json:"amount,omitempty"`
	Currency  string `json:"currency"`
	SalePrice string `json:"sale_price,omitempty"`
}

type PartialEditInventory struct {
	WarehouseID string `json:"warehouse_id"`
	Quantity    int    `json:"quantity"`
}

type PartialEditProductResponse struct {
	ProductID string              `json:"product_id"`
	SKUs      []PartialEditSKURes `json:"skus"`
}

type PartialEditSKURes struct {
	ID        string `json:"id"`
	SellerSKU string `json:"seller_sku"`
}

// PartialEditProduct updates SKUs of a product using Partial Edit API
// NOTE: Must include ALL existing SKUs + new SKUs, otherwise missing SKUs will be DELETED
func (p *ProductsAPI) PartialEditProduct(ctx context.Context, shopID uint, shopCipher, productID string, req *PartialEditProductRequest) (*PartialEditProductResponse, error) {
	log := logger.Log.Named("tiktok_api")

	accessToken, err := p.tokenManager.GetValidToken(ctx, shopID)
	if err != nil {
		return nil, err
	}

	log.Debug("partial edit request",
		zap.String("product_id", productID),
		zap.Int("sku_count", len(req.SKUs)))

	resp, err := p.client.Request(ctx, http.MethodPost, fmt.Sprintf("/product/202309/products/%s/partial_edit", productID), nil, &RequestOption{
		AccessToken: accessToken,
		ShopCipher:  shopCipher,
		Body:        req,
	})
	if err != nil {
		log.Error("partial edit failed",
			zap.String("product_id", productID),
			zap.Error(err))
		return nil, err
	}

	var result PartialEditProductResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse partial edit response: %w", err)
	}

	log.Info("partial edit success",
		zap.String("product_id", productID),
		zap.Int("returned_skus", len(result.SKUs)))

	return &result, nil
}
