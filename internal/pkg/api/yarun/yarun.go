package yarun

import (
	"context"
	"strconv"
	"time"

	"github.com/imroc/req/v3"
)

// YarunApi represents the yarun API client
type YarunApi struct {
	client *req.Client
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
	client := req.C().
		SetBaseURL(baseURL).
		SetCommonHeader("x-token", token).
		SetCommonHeader("Content-Type", "application/json").
		SetTimeout(30 * time.Second)

	return &YarunApi{
		client: client,
	}
}

// GetProxies gets n proxies from the API with round robin functionality
func (y *YarunApi) GetProxies(ctx context.Context, limit int) (*GetProxiesResponse, error) {
	var response GetProxiesResponse

	resp, err := y.client.R().
		SetContext(ctx).
		SetQueryParam("limit", strconv.Itoa(limit)).
		SetSuccessResult(&response).
		Get("/proxy")

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccessState() {
		return nil, resp.Err
	}

	return &response, nil
}

// BlockProxy sets a 1-hour cooldown on a specific proxy
func (y *YarunApi) BlockProxy(ctx context.Context, proxyID string) (*BlockProxyResponse, error) {
	request := BlockProxyRequest{
		ID: proxyID,
	}

	var response BlockProxyResponse

	resp, err := y.client.R().
		SetContext(ctx).
		SetBodyJsonMarshal(request).
		SetSuccessResult(&response).
		Post("/proxy/blocked")

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccessState() {
		return nil, resp.Err
	}

	return &response, nil
}
