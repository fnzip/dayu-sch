package cfbatch

import (
	"context"
	"net"

	"github.com/imroc/req/v3"
)

type CFBatchApi struct {
	client *req.Client
}

func NewCFBatchApi(baseUrl, token string) *CFBatchApi {
	client := req.C().
		SetCommonHeader("x-token", token).
		SetBaseURL(baseUrl)

	client.DevMode()

	return &CFBatchApi{
		client: client,
	}
}

func (a *CFBatchApi) SetDialContext(dialContext func(ctx context.Context, network string, addr string) (net.Conn, error)) {
	a.client.DialContext = dialContext
}

func (a *CFBatchApi) SendBatch(ctx context.Context, users []CFBatchUser) error {
	rb := CFBatchUserBody{
		Users: users,
	}

	_, err := a.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(rb).
		Post("/batch")
	if err != nil {
		return err
	}

	return nil
}
