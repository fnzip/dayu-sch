package cfbatch_v2

import (
	"context"
	"fmt"

	"github.com/imroc/req/v3"
)

type CFBatchApi struct {
	client *req.Client
}

type BatchResult struct {
	Balance float64 `json:"b"` // b = balance
	Coin    float64 `json:"c"` // c = coin
}

type BatchResponse struct {
	App      string      `json:"a"` // a = app
	Username string      `json:"u"` // u = username
	Status   bool        `json:"s"` // s = status
	Result   BatchResult `json:"r"` // r = result
}

func NewCFBatchApi(baseUrl, token string) *CFBatchApi {
	client := req.C().
		SetCommonHeader("x-token", token).
		SetCommonHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36").
		SetBaseURL(baseUrl)

	// client.DevMode()

	return &CFBatchApi{
		client: client,
	}
}

func (a *CFBatchApi) Clone() *CFBatchApi {
	return &CFBatchApi{
		client: a.client.Clone(),
	}
}

func (a *CFBatchApi) SetProxyURL(proxyURL string) {
	a.client.SetProxyURL(proxyURL)
}

func (a *CFBatchApi) SendBatch(ctx context.Context, limit int) ([]BatchResponse, error) {
	var response []BatchResponse

	resp, err := a.client.R().
		SetContext(ctx).
		SetQueryParam("limit", fmt.Sprintf("%d", limit)).
		SetSuccessResult(&response).
		Post("/batch")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return response, nil
}
