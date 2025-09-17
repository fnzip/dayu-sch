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
	ID          string    `json:"_id"`
	Port        int       `json:"port"`
	ReleaseAt   time.Time `json:"release_at"`
	IP          string    `json:"ip"`
	IsBlocked   bool      `json:"is_blocked"`
	LastCheckAt time.Time `json:"last_check_at"`
}

// GetProxiesResponse represents the response from GET /proxy
type GetProxiesResponse struct {
	Ok      bool            `json:"ok"`
	Proxies []ProxyResponse `json:"proxies"`
}

// GetBlockedProxiesResponse represents the response from GET /proxy/blocked
type GetBlockedProxiesResponse struct {
	Ok      bool            `json:"ok"`
	Proxies []ProxyResponse `json:"proxies"`
}

// BlockProxyRequest represents the request to block a proxy
type BlockProxyRequest struct {
	ID string `json:"_id"`
}

// BlockProxyResponse represents the response from POST /proxy/blocked
type BlockProxyResponse struct {
	Ok bool `json:"ok"`
}

// UnblockProxyRequest represents the request to unblock a proxy
type UnblockProxyRequest struct {
	ID        string `json:"_id"`
	IP        string `json:"ip"`
	IsBlocked bool   `json:"is_blocked"`
}

// UnblockProxyResponse represents the response from POST /proxy/unblock
type UnblockProxyResponse struct {
	Ok bool `json:"ok"`
}

// UpdateBalanceRequest represents the request to update user balance
type UpdateBalanceRequest struct {
	ID      string  `json:"_id"`
	Balance float64 `json:"balance"`
	Coin    float64 `json:"coin"`
}

// UpdateBalanceResponse represents the response from POST /user/balance
type UpdateBalanceResponse struct {
	Ok bool `json:"ok"`
}

// NewYarunApi creates a new yarun API client
func NewYarunApi(baseURL, token string) *YarunApi {
	client := req.C().
		SetBaseURL(baseURL).
		SetCommonHeader("x-token", token).
		SetCommonHeader("Content-Type", "application/json").
		SetTimeout(30 * time.Second)

	// client.DevMode()

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

// GetBlockedProxies gets blocked proxies from the API
func (y *YarunApi) GetBlockedProxies(ctx context.Context, limit int) (*GetBlockedProxiesResponse, error) {
	var response GetBlockedProxiesResponse

	resp, err := y.client.R().
		SetContext(ctx).
		SetQueryParam("limit", strconv.Itoa(limit)).
		SetSuccessResult(&response).
		Get("/proxy/blocked")

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccessState() {
		return nil, resp.Err
	}

	return &response, nil
}

// UnblockProxy unblocks a proxy and updates its IP
func (y *YarunApi) UnblockProxy(ctx context.Context, proxyID, newIP string, isBlocked bool) (*UnblockProxyResponse, error) {
	request := UnblockProxyRequest{
		ID:        proxyID,
		IP:        newIP,
		IsBlocked: isBlocked,
	}

	var response UnblockProxyResponse

	resp, err := y.client.R().
		SetContext(ctx).
		SetBodyJsonMarshal(request).
		SetSuccessResult(&response).
		Post("/proxy/unblock")

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccessState() {
		return nil, resp.Err
	}

	return &response, nil
}

func (y *YarunApi) UpdateUserBalance(ctx context.Context, userID string, balance, coin float64) (*UpdateBalanceResponse, error) {
	request := UpdateBalanceRequest{
		ID:      userID,
		Balance: balance,
		Coin:    coin,
	}

	var response UpdateBalanceResponse

	resp, err := y.client.R().
		SetContext(ctx).
		SetBodyJsonMarshal(request).
		SetSuccessResult(&response).
		Post("/user/balance")

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccessState() {
		return nil, resp.Err
	}

	return &response, nil
}
