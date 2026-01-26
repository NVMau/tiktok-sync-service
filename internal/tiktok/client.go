package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/pkg/signature"
)

type Client struct {
	httpClient  *http.Client
	cfg         *config.Config
	rateLimiter *RateLimiter
}

type RateLimiter struct {
	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	return &RateLimiter{
		minInterval: time.Second / time.Duration(requestsPerSecond),
	}
}

func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRequest)
	if elapsed < r.minInterval {
		time.Sleep(r.minInterval - elapsed)
	}
	r.lastRequest = time.Now()
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cfg:         cfg,
		rateLimiter: NewRateLimiter(10),
	}
}

type APIResponse struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

type RequestOption struct {
	AccessToken string
	ShopCipher  string
	Body        interface{}
}

func (c *Client) Request(ctx context.Context, method, path string, params map[string]string, opt *RequestOption) (*APIResponse, error) {
	c.rateLimiter.Wait()

	if params == nil {
		params = make(map[string]string)
	}

	params["app_key"] = c.cfg.TikTokAppKey
	params["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)

	if opt != nil && opt.ShopCipher != "" {
		params["shop_cipher"] = opt.ShopCipher
	}

	var bodyBytes []byte
	var err error
	if opt != nil && opt.Body != nil {
		bodyBytes, err = json.Marshal(opt.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		log.Printf("Request body: %s", string(bodyBytes))
	}

	sign := signature.GenerateSign(c.cfg.TikTokAppSecret, path, params, bodyBytes)
	params["sign"] = sign

	if opt != nil && opt.AccessToken != "" {
		params["access_token"] = opt.AccessToken
	}

	fullURL := c.buildURL(path, params)
	log.Printf("Request URL: %s %s", method, fullURL)
	log.Printf("Body length: %d", len(bodyBytes))

	var req *http.Request
	if method == http.MethodGet || len(bodyBytes) == 0 {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(bodyBytes))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if opt != nil && opt.AccessToken != "" {
		req.Header.Set("x-tts-access-token", opt.AccessToken)
	}

	resp, err := c.doWithRetry(req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	if apiResp.Code != 0 {
		return &apiResp, fmt.Errorf("API error: code=%d, message=%s", apiResp.Code, apiResp.Message)
	}

	return &apiResp, nil
}

func (c *Client) buildURL(path string, params map[string]string) string {
	u, _ := url.Parse(c.cfg.TikTokAPIBaseURL + path)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					time.Sleep(time.Duration(seconds) * time.Second)
					continue
				}
			}
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
