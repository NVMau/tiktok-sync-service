package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestGenerateSign(t *testing.T) {
	tests := []struct {
		name      string
		appSecret string
		path      string
		params    map[string]string
		body      []byte
		wantLen   int
	}{
		{
			name:      "basic sign without body",
			appSecret: "e59af819cc",
			path:      "/authorization/202309/shops",
			params: map[string]string{
				"app_key":   "123abc",
				"timestamp": "1699999999",
			},
			body:    nil,
			wantLen: 64,
		},
		{
			name:      "sign with body",
			appSecret: "e59af819cc",
			path:      "/order/202309/orders",
			params: map[string]string{
				"app_key":     "123abc",
				"timestamp":   "1699999999",
				"shop_cipher": "ROW_xxx",
			},
			body:    []byte(`{"order_id":"12345"}`),
			wantLen: 64,
		},
		{
			name:      "excludes sign and access_token params",
			appSecret: "e59af819cc",
			path:      "/test",
			params: map[string]string{
				"app_key":      "123abc",
				"timestamp":    "1699999999",
				"sign":         "should_be_excluded",
				"access_token": "should_be_excluded",
			},
			body:    nil,
			wantLen: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSign(tt.appSecret, tt.path, tt.params, tt.body)
			if len(got) != tt.wantLen {
				t.Errorf("GenerateSign() length = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestGenerateSign_Deterministic(t *testing.T) {
	appSecret := "test_secret"
	path := "/test/path"
	params := map[string]string{
		"app_key":   "key123",
		"timestamp": "1699999999",
		"param_a":   "value_a",
		"param_b":   "value_b",
	}
	body := []byte(`{"test":"data"}`)

	sign1 := GenerateSign(appSecret, path, params, body)
	sign2 := GenerateSign(appSecret, path, params, body)

	if sign1 != sign2 {
		t.Errorf("GenerateSign() not deterministic: %s != %s", sign1, sign2)
	}
}

func TestGenerateSign_ParamsOrder(t *testing.T) {
	appSecret := "test_secret"
	path := "/test"

	params1 := map[string]string{
		"zebra": "1",
		"apple": "2",
		"mango": "3",
	}

	params2 := map[string]string{
		"apple": "2",
		"mango": "3",
		"zebra": "1",
	}

	sign1 := GenerateSign(appSecret, path, params1, nil)
	sign2 := GenerateSign(appSecret, path, params2, nil)

	if sign1 != sign2 {
		t.Errorf("GenerateSign() should be order-independent: %s != %s", sign1, sign2)
	}
}

func generateTestWebhookSignature(secret, timestamp string, body []byte) string {
	signedPayload := timestamp + "." + string(body)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedPayload))
	return hex.EncodeToString(h.Sum(nil))
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "my_webhook_secret"
	timestamp := "1633174587"
	body := []byte(`{"type":"ORDER_STATUS_CHANGE","shop_id":"123"}`)

	validSig := generateTestWebhookSignature(secret, timestamp, body)

	tests := []struct {
		name      string
		secret    string
		timestamp string
		body      []byte
		signature string
		want      bool
	}{
		{
			name:      "valid signature",
			secret:    secret,
			timestamp: timestamp,
			body:      body,
			signature: validSig,
			want:      true,
		},
		{
			name:      "invalid signature",
			secret:    secret,
			timestamp: timestamp,
			body:      body,
			signature: "invalid_signature",
			want:      false,
		},
		{
			name:      "wrong secret",
			secret:    "wrong_secret",
			timestamp: timestamp,
			body:      body,
			signature: validSig,
			want:      false,
		},
		{
			name:      "modified body",
			secret:    secret,
			timestamp: timestamp,
			body:      []byte(`{"type":"MODIFIED"}`),
			signature: validSig,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifyWebhookSignature(tt.secret, tt.timestamp, tt.body, tt.signature)
			if got != tt.want {
				t.Errorf("VerifyWebhookSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseWebhookSignatureHeader(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		wantTimestamp string
		wantSignature string
	}{
		{
			name:          "valid header",
			header:        "t=1633174587,s=18494715036ac4416a1d0a673871a2edbcfc94d94bd88ccd2c5ec9b3425afe66",
			wantTimestamp: "1633174587",
			wantSignature: "18494715036ac4416a1d0a673871a2edbcfc94d94bd88ccd2c5ec9b3425afe66",
		},
		{
			name:          "header with spaces",
			header:        "t = 1633174587 , s = abc123",
			wantTimestamp: "1633174587",
			wantSignature: "abc123",
		},
		{
			name:          "empty header",
			header:        "",
			wantTimestamp: "",
			wantSignature: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTimestamp, gotSignature := ParseWebhookSignatureHeader(tt.header)
			if gotTimestamp != tt.wantTimestamp {
				t.Errorf("ParseWebhookSignatureHeader() timestamp = %v, want %v", gotTimestamp, tt.wantTimestamp)
			}
			if gotSignature != tt.wantSignature {
				t.Errorf("ParseWebhookSignatureHeader() signature = %v, want %v", gotSignature, tt.wantSignature)
			}
		})
	}
}
