package yarun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// YarunApi represents the yarun API client
type YarunApi struct {
	baseURL string
	token   string
	client  *http.Client
}

// ProxyResponse represents a proxy object
type ProxyResponse struct {
	Port      int       `json:"port"`
	ReleaseAt time.Time `json:"release_at"`
}

// GetProxiesResponse represents the response from GET /proxy
type GetProxiesResponse struct {
	Ok      bool            `json:"ok"`
	Proxies []ProxyResponse `json:"proxies"`
}

// BlockProxyRequest represents the request to block a proxy
type BlockProxyRequest struct {
	ID string `json:"id"`
}

// BlockProxyResponse represents the response from POST /proxy/blocked
type BlockProxyResponse struct {
	Ok bool `json:"ok"`
}

// NewYarunApi creates a new yarun API client
func NewYarunApi(baseURL, token string) *YarunApi {
	return &YarunApi{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxies gets n proxies from the API with round robin functionality
func (y *YarunApi) GetProxies(ctx context.Context, limit int) (*GetProxiesResponse, error) {
	url := fmt.Sprintf("%s/proxy?limit=%d", y.baseURL, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-token", y.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response GetProxiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// BlockProxy sets a 1-hour cooldown on a specific proxy
func (y *YarunApi) BlockProxy(ctx context.Context, proxyID string) (*BlockProxyResponse, error) {
	url := fmt.Sprintf("%s/proxy/blocked", y.baseURL)

	request := BlockProxyRequest{
		ID: proxyID,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-token", y.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response BlockProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
