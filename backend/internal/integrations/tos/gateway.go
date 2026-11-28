package tos

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/telemetry"
	sdk "github.com/volcengine/ve-tos-golang-sdk/v2/tos"
)

type tracedTransport struct {
	*sdk.DefaultTransport
}

func (t tracedTransport) RoundTrip(ctx context.Context, req *sdk.Request) (*sdk.Response, error) {
	ctx, span := telemetry.StartHTTP(ctx, "volcengine.tos", req.Method)
	resp, err := t.DefaultTransport.RoundTrip(ctx, req)
	if resp == nil {
		telemetry.Finish(span, err)
	} else {
		resp.Body = telemetry.ResponseBody(span, resp.StatusCode, resp.Body, err)
	}

	return resp, err
}

type Gateway struct {
	client   *sdk.ClientV2
	endpoint string
}

func New(storage config.ObjectStorage) (*Gateway, error) {
	if !storage.Configured() {
		return nil, nil
	}

	if err := storage.Validate(); err != nil {
		return nil, err
	}

	transport := sdk.DefaultTransportConfig()
	transport.ProxyFunc = http.ProxyFromEnvironment
	transport.ResponseHeaderTimeout = 30 * time.Second
	client, err := sdk.NewClientV2(storage.Endpoint,
		sdk.WithRegion(storage.Region),
		sdk.WithCredentials(sdk.NewStaticCredentials(storage.AccessKey, storage.SecretKey)),
		sdk.WithTransport(tracedTransport{sdk.NewDefaultTransport(&transport)}),
		sdk.WithMaxRetryCount(1),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化火山 TOS 官方 SDK 失败")
	}

	return &Gateway{
		client:   client,
		endpoint: storage.Endpoint,
	}, nil
}
