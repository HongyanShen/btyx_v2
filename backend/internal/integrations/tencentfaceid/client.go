package tencentfaceid

import (
	"context"
	"errors"
	"net/http"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	faceid "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/faceid/v20180301"
)

type Client struct {
	api interface {
		IdCardOCRVerificationWithContext(context.Context, *faceid.IdCardOCRVerificationRequest) (*faceid.IdCardOCRVerificationResponse, error)
	}
}

func New(secretID, secretKey string) (*Client, error) {
	if secretID == "" || secretKey == "" {
		return nil, nil
	}

	client, err := faceid.NewClient(common.NewCredential(secretID, secretKey), "", profile.NewClientProfile())
	if err != nil {
		return nil, errors.New("初始化实名信息核验失败")
	}

	return &Client{api: client}, nil
}

func (c *Client) Check(ctx context.Context, name, number string) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	request := faceid.NewIdCardOCRVerificationRequest()
	request.Name = common.StringPtr(name)
	request.IdCard = common.StringPtr(number)

	response, err := c.api.IdCardOCRVerificationWithContext(ctx, request)
	if err != nil {
		var sdkError *tcerrors.TencentCloudSDKError
		if errors.As(err, &sdkError) {
			switch sdkError.Code {
			case faceid.UNAUTHORIZEDOPERATION_NONACTIVATED:
				return unavailable("实名信息核验服务尚未开通")
			case "AuthFailure.UnauthorizedOperation":
				return unavailable("实名信息核验服务尚未授权")
			}
		}

		return unavailable("实名信息核验暂不可用，请稍后重试")
	}

	if response == nil || response.Response == nil || response.Response.Result == nil {
		return unavailable("实名信息核验暂未返回结果，请稍后重试")
	}

	switch *response.Response.Result {
	case "0":
		return nil
	case "-1":
		return app.Invalid("姓名和身份证号不一致，请核对后重试")
	case "-2":
		return app.Invalid("身份证号无效，请核对后重试")
	case "-3":
		return app.Invalid("姓名格式无效，请核对后重试")
	case "-5":
		return app.Invalid("未查到该身份证记录，请核对后重试")
	case "-7":
		return unavailable("已达到当日实名核验次数限制，请次日重试")
	default:
		return unavailable("实名信息核验暂不可用，请稍后重试")
	}
}

func unavailable(message string) error {
	return &app.Error{
		Status:  http.StatusServiceUnavailable,
		Code:    enum.ErrorUnavailable,
		Message: message,
	}
}
