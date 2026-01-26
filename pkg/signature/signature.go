package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

func GenerateSign(appSecret, path string, params map[string]string, body []byte) string {
	excludeKeys := map[string]bool{
		"sign":         true,
		"access_token": true,
	}

	var keys []string
	for k := range params {
		if !excludeKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var paramStr strings.Builder
	for _, k := range keys {
		paramStr.WriteString(k)
		paramStr.WriteString(params[k])
	}

	var signStr strings.Builder
	signStr.WriteString(appSecret)
	signStr.WriteString(path)
	signStr.WriteString(paramStr.String())
	if len(body) > 0 {
		signStr.Write(body)
	}
	signStr.WriteString(appSecret)

	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write([]byte(signStr.String()))
	return hex.EncodeToString(h.Sum(nil))
}

func VerifyWebhookSignature(secret, timestamp string, body []byte, signature string) bool {
	signedPayload := timestamp + "." + string(body)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(signature))
}

func ParseWebhookSignatureHeader(header string) (timestamp, signature string) {
	parts := strings.Split(header, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		switch key {
		case "t":
			timestamp = value
		case "s":
			signature = value
		}
	}
	return
}
