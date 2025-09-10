package cfbatch

import (
	"context"

	"github.com/imroc/req/v3"
)

type CFBatchApi struct {
	client *req.Client
}

func NewCFBatchApi(baseUrl, token string) *CFBatchApi {
	client := req.C().
		SetCommonHeader("x-token", token).
		SetBaseURL(baseUrl)

	return &CFBatchApi{
		client: client,
	}
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
