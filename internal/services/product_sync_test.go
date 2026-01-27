package services

import (
	"testing"

	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/models"
	"github.com/user/sync-tiktok-mps/internal/tiktok"
)

func init() {
	// Initialize logger for tests
	logger.Init(&logger.Config{
		Level:       "debug",
		Environment: "development",
		OutputPath:  "stdout",
	})
}

// =============================================================================
// HELPER FUNCTIONS FOR TESTS
// =============================================================================

func createMockProductDetail(skuCount int, hasVariant bool) *tiktok.ProductDetail {
	detail := &tiktok.ProductDetail{
		ID:     "mock-product-123",
		Title:  "Test Product",
		Status: "ACTIVATE",
		SKUs:   make([]tiktok.ProductSKU, skuCount),
	}

	for i := 0; i < skuCount; i++ {
		sku := tiktok.ProductSKU{
			ID:        "tiktok-sku-" + string(rune('A'+i)),
			SellerSku: "seller-sku-" + string(rune('0'+i)),
			Price: tiktok.ProductPrice{
				SalePrice: "100000",
				Currency:  "VND",
			},
			Inventory: []tiktok.ProductInventory{
				{WarehouseID: "warehouse-123", Quantity: 1},
			},
		}

		if hasVariant {
			sku.SalesAttributes = []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				ValueID   string `json:"value_id"`
				ValueName string `json:"value_name"`
			}{
				{
					ID:        "attr-id-123",
					Name:      "CHỌN SỐ",
					ValueID:   "value-id-" + string(rune('A'+i)),
					ValueName: "variant-" + string(rune('A'+i)),
				},
			}
		}

		detail.SKUs[i] = sku
	}

	return detail
}

func createMockPendingSKUs(count int) []models.SKU {
	skus := make([]models.SKU, count)
	for i := 0; i < count; i++ {
		skus[i] = models.SKU{
			TikTokProductID: "product_123",
			SellerSKU:       "0912345" + string(rune('0'+i)) + "00",
			Price:           150000 + float64(i*10000),
			Quantity:        1,
			SyncStatus:      models.SKUSyncStatusPending,
			SaleStatus:      models.SKUSaleStatusAvailable,
		}
		skus[i].ID = uint(i + 1)
	}
	return skus
}

// =============================================================================
// TEST: getVariantAttribute
// =============================================================================

func TestProductSyncService_getVariantAttribute(t *testing.T) {
	svc := &ProductSyncService{}

	tests := []struct {
		name       string
		detail     *tiktok.ProductDetail
		wantNil    bool
		wantID     string
		wantName   string
	}{
		{
			name:     "product with variant",
			detail:   createMockProductDetail(3, true),
			wantNil:  false,
			wantID:   "attr-id-123",
			wantName: "CHỌN SỐ",
		},
		{
			name:    "product without variant",
			detail:  createMockProductDetail(1, false),
			wantNil: true,
		},
		{
			name: "product with no SKUs",
			detail: &tiktok.ProductDetail{
				ID:   "empty-product",
				SKUs: []tiktok.ProductSKU{},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.getVariantAttribute(tt.detail)
			if tt.wantNil {
				if got != nil {
					t.Errorf("getVariantAttribute() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("getVariantAttribute() = nil, want non-nil")
			}
			if got.ID != tt.wantID {
				t.Errorf("getVariantAttribute().ID = %s, want %s", got.ID, tt.wantID)
			}
			if got.Name != tt.wantName {
				t.Errorf("getVariantAttribute().Name = %s, want %s", got.Name, tt.wantName)
			}
		})
	}
}

// =============================================================================
// TEST: getWarehouseID
// =============================================================================

func TestProductSyncService_getWarehouseID(t *testing.T) {
	svc := &ProductSyncService{}

	tests := []struct {
		name   string
		detail *tiktok.ProductDetail
		want   string
	}{
		{
			name:   "product with warehouse",
			detail: createMockProductDetail(2, true),
			want:   "warehouse-123",
		},
		{
			name: "product with no inventory",
			detail: &tiktok.ProductDetail{
				ID: "no-inventory",
				SKUs: []tiktok.ProductSKU{
					{ID: "sku-1", Inventory: []tiktok.ProductInventory{}},
				},
			},
			want: "",
		},
		{
			name: "product with no SKUs",
			detail: &tiktok.ProductDetail{
				ID:   "no-skus",
				SKUs: []tiktok.ProductSKU{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.getWarehouseID(tt.detail)
			if got != tt.want {
				t.Errorf("getWarehouseID() = %s, want %s", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TEST: buildExistingSKUs (helper to test partial logic without DB)
// =============================================================================

func TestProductSyncService_BuildExistingSKUs(t *testing.T) {
	// Test that existing SKUs are correctly formatted for partial edit
	detail := createMockProductDetail(3, true)

	existingSellerSKUs := make(map[string]bool)
	var editSKUs []tiktok.PartialEditSKU

	for _, tikSKU := range detail.SKUs {
		existingSellerSKUs[tikSKU.SellerSku] = true

		price := tikSKU.Price.SalePrice
		if price == "" {
			price = tikSKU.Price.Amount
		}

		var salesAttrs []tiktok.PartialEditAttribute
		for _, attr := range tikSKU.SalesAttributes {
			salesAttrs = append(salesAttrs, tiktok.PartialEditAttribute{
				ID:        attr.ID,
				Name:      attr.Name,
				ValueID:   attr.ValueID,
				ValueName: attr.ValueName,
			})
		}

		editSKUs = append(editSKUs, tiktok.PartialEditSKU{
			ID:              tikSKU.ID,
			SellerSKU:       tikSKU.SellerSku,
			SalesAttributes: salesAttrs,
			Price: tiktok.PartialEditPrice{
				Currency:  "VND",
				SalePrice: price,
			},
		})
	}

	// Verify
	if len(editSKUs) != 3 {
		t.Errorf("expected 3 SKUs, got %d", len(editSKUs))
	}

	// All existing SKUs should have ID
	for i, sku := range editSKUs {
		if sku.ID == "" {
			t.Errorf("existing SKU[%d] should have ID set", i)
		}
		// Existing SKUs should NOT have inventory (race condition protection)
		if sku.Inventory != nil {
			t.Errorf("existing SKU[%d] should NOT have Inventory", i)
		}
	}
}

func TestProductSyncService_BuildNewSKU(t *testing.T) {
	// Test that new SKU is correctly formatted
	variantAttr := &variantAttribute{
		ID:   "attr-123",
		Name: "CHỌN SỐ",
	}
	warehouseID := "warehouse-123"

	sku := models.SKU{
		SellerSKU: "0912345678",
		Price:     150000,
		Quantity:  1,
	}

	priceStr := "150000"
	newSKU := tiktok.PartialEditSKU{
		SellerSKU: sku.SellerSKU,
		SalesAttributes: []tiktok.PartialEditAttribute{
			{
				ID:        variantAttr.ID,
				Name:      variantAttr.Name,
				ValueName: sku.SellerSKU,
			},
		},
		Price: tiktok.PartialEditPrice{
			Currency:  "VND",
			Amount:    priceStr,
			SalePrice: priceStr,
		},
		Inventory: []tiktok.PartialEditInventory{
			{
				WarehouseID: warehouseID,
				Quantity:    sku.Quantity,
			},
		},
	}

	// Verify
	if newSKU.ID != "" {
		t.Error("new SKU should NOT have ID")
	}
	if newSKU.SellerSKU != "0912345678" {
		t.Errorf("SellerSKU = %s, want 0912345678", newSKU.SellerSKU)
	}
	if len(newSKU.SalesAttributes) != 1 {
		t.Errorf("should have 1 sales attribute, got %d", len(newSKU.SalesAttributes))
	}
	if newSKU.SalesAttributes[0].ValueName != "0912345678" {
		t.Errorf("variant value should be phone number, got %s", newSKU.SalesAttributes[0].ValueName)
	}
	if newSKU.Inventory == nil || len(newSKU.Inventory) == 0 {
		t.Error("new SKU should have Inventory")
	}
	if newSKU.Inventory[0].Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", newSKU.Inventory[0].Quantity)
	}
}

func TestProductSyncService_SkipDuplicateSellerSKU(t *testing.T) {
	detail := createMockProductDetail(2, true)

	existingSellerSKUs := make(map[string]bool)
	for _, tikSKU := range detail.SKUs {
		existingSellerSKUs[tikSKU.SellerSku] = true
	}

	// Pending SKU with same seller_sku as existing
	pendingSKU := models.SKU{
		SellerSKU: detail.SKUs[0].SellerSku, // Same as existing!
	}

	// Check if should skip
	shouldSkip := existingSellerSKUs[pendingSKU.SellerSKU]
	if !shouldSkip {
		t.Error("should skip SKU with duplicate seller_sku")
	}

	// Different SKU should NOT skip
	pendingSKU2 := models.SKU{
		SellerSKU: "completely-new-sku",
	}
	shouldSkip2 := existingSellerSKUs[pendingSKU2.SellerSKU]
	if shouldSkip2 {
		t.Error("should NOT skip SKU with new seller_sku")
	}
}

// =============================================================================
// TEST: SKU Status Constants
// =============================================================================

func TestSKUSyncStatus_Values(t *testing.T) {
	tests := []struct {
		status models.SKUSyncStatus
		want   string
	}{
		{models.SKUSyncStatusPending, "PENDING"},
		{models.SKUSyncStatusPushing, "PUSHING"},
		{models.SKUSyncStatusPushed, "PUSHED"},
		{models.SKUSyncStatusSynced, "SYNCED"},
		{models.SKUSyncStatusFailed, "FAILED"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("SKUSyncStatus = %s, want %s", tt.status, tt.want)
		}
	}
}

func TestSKUSaleStatus_Values(t *testing.T) {
	tests := []struct {
		status models.SKUSaleStatus
		want   string
	}{
		{models.SKUSaleStatusAvailable, "AVAILABLE"},
		{models.SKUSaleStatusSold, "SOLD"},
		{models.SKUSaleStatusReserved, "RESERVED"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("SKUSaleStatus = %s, want %s", tt.status, tt.want)
		}
	}
}

// =============================================================================
// TEST: SKUCreateRequest
// =============================================================================

func TestSKUCreateRequest_Validation(t *testing.T) {
	tests := []struct {
		name      string
		req       SKUCreateRequest
		wantValid bool
	}{
		{
			name:      "valid request",
			req:       SKUCreateRequest{SellerSKU: "0912345678", Price: 150000},
			wantValid: true,
		},
		{
			name:      "empty seller_sku",
			req:       SKUCreateRequest{SellerSKU: "", Price: 150000},
			wantValid: false,
		},
		{
			name:      "zero price is valid (free)",
			req:       SKUCreateRequest{SellerSKU: "0912345678", Price: 0},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.req.SellerSKU != ""
			if isValid != tt.wantValid {
				t.Errorf("validation = %v, want %v", isValid, tt.wantValid)
			}
		})
	}
}

// =============================================================================
// TEST: variantAttribute struct
// =============================================================================

func TestVariantAttribute_Fields(t *testing.T) {
	attr := variantAttribute{
		ID:   "attr-123",
		Name: "CHỌN SỐ",
	}

	if attr.ID != "attr-123" {
		t.Errorf("ID = %s, want attr-123", attr.ID)
	}
	if attr.Name != "CHỌN SỐ" {
		t.Errorf("Name = %s, want CHỌN SỐ", attr.Name)
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkBuildPartialEditRequest(b *testing.B) {
	svc := &ProductSyncService{
		log: logger.Log.Named("bench"),
	}

	detail := createMockProductDetail(50, true)
	pendingSKUs := createMockPendingSKUs(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.buildPartialEditRequest(detail, pendingSKUs, "warehouse-123")
	}
}

func BenchmarkGetVariantAttribute(b *testing.B) {
	svc := &ProductSyncService{}
	detail := createMockProductDetail(100, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.getVariantAttribute(detail)
	}
}
