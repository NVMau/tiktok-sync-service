package masker

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Sensitive field names to mask in JSON
var sensitiveFields = []string{
	"access_token",
	"refresh_token",
	"phone",
	"phone_number",
	"address",
	"recipient_address",
	"seller_sku",
	"password",
	"secret",
	"token",
}

// Regex patterns for sensitive data
var (
	phoneRegex = regexp.MustCompile(`\b(0\d{9,10}|\+84\d{9,10})\b`)
	tokenRegex = regexp.MustCompile(`(access_token|token)=[^&\s"]+`)
)

// MaskJSON masks sensitive fields in JSON data
func MaskJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		// Not valid JSON, mask as string
		return MaskString(string(data))
	}

	maskMap(obj)

	result, err := json.Marshal(obj)
	if err != nil {
		return "[MASK_ERROR]"
	}
	return string(result)
}

// MaskString masks sensitive patterns in a string
func MaskString(s string) string {
	// Mask phone numbers
	s = phoneRegex.ReplaceAllStringFunc(s, func(match string) string {
		if len(match) <= 4 {
			return match
		}
		return match[:3] + "****" + match[len(match)-2:]
	})

	// Mask tokens in URLs
	s = tokenRegex.ReplaceAllStringFunc(s, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=****"
		}
		return match
	})

	return s
}

// MaskURL masks sensitive query params in URLs
func MaskURL(url string) string {
	sensitiveParams := []string{"access_token", "token", "secret", "password"}
	result := url

	for _, param := range sensitiveParams {
		pattern := regexp.MustCompile(param + `=[^&\s]+`)
		result = pattern.ReplaceAllString(result, param+"=****")
	}

	return result
}

func maskMap(m map[string]interface{}) {
	for key, value := range m {
		keyLower := strings.ToLower(key)

		// Check if this field should be masked
		shouldMask := false
		for _, sensitive := range sensitiveFields {
			if strings.Contains(keyLower, sensitive) {
				shouldMask = true
				break
			}
		}

		if shouldMask {
			if str, ok := value.(string); ok {
				m[key] = maskValue(str)
			}
			continue
		}

		// Recursively process nested objects
		switch v := value.(type) {
		case map[string]interface{}:
			maskMap(v)
		case []interface{}:
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					maskMap(itemMap)
				}
			}
		case string:
			// Mask phone numbers in any string value
			m[key] = phoneRegex.ReplaceAllStringFunc(v, func(match string) string {
				if len(match) <= 4 {
					return match
				}
				return match[:3] + "****" + match[len(match)-2:]
			})
		}
	}
}

func maskValue(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	if len(s) <= 8 {
		return s[:2] + "****"
	}
	return s[:4] + "****" + s[len(s)-2:]
}
