package tos

import (
	"fmt"
	"net/http"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/config"
	sdk "github.com/volcengine/ve-tos-golang-sdk/v2/tos"
)

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

	client, err := sdk.NewClientV2(storage.Endpoint,
		sdk.WithRegion(storage.Region),
		sdk.WithCredentials(sdk.NewStaticCredentials(storage.AccessKey, storage.SecretKey)),
		sdk.WithProxyFunc(http.ProxyFromEnvironment),
		sdk.WithRequestTimeout(30*time.Second),
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
