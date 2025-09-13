package cfbatch_v2

import (
	"context"
	"fmt"

	"github.com/imroc/req/v3"
)

type CFBatchApi struct {
	client *req.Client
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

func (a *CFBatchApi) SendBatch(ctx context.Context, limit int) error {
	_, err := a.client.R().
		SetContext(ctx).
		SetQueryParam("limit", fmt.Sprintf("%d", limit)).
		Post("/batch")
	if err != nil {
		return err
	}

	return nil
}
