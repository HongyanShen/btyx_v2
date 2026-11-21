package tencentocr

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
	ocr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ocr/v20181119"
)

type Client struct {
	ocr *ocr.Client
}

func New(secretID, secretKey string) (*Client, error) {
	if secretID == "" || secretKey == "" {
		return nil, nil
	}

	client, err := ocr.NewClient(common.NewCredential(secretID, secretKey), "", profile.NewClientProfile())
	if err != nil {
		return nil, errors.New("初始化身份证自动识别失败")
	}

	return &Client{ocr: client}, nil
}

func (c *Client) Recognize(ctx context.Context, imageURL string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	request := ocr.NewIDCardOCRRequest()
	request.ImageUrl = common.StringPtr(imageURL)
	request.CardSide = common.StringPtr("FRONT")

	response, err := c.ocr.IDCardOCRWithContext(ctx, request)
	if err != nil {
		var apiError *tcerrors.TencentCloudSDKError
		if errors.As(err, &apiError) {
			switch apiError.Code {
			case ocr.FAILEDOPERATION_CARDSIDEERROR,
				ocr.FAILEDOPERATION_IMAGEBLUR,
				ocr.FAILEDOPERATION_IMAGEDECODEFAILED,
				ocr.FAILEDOPERATION_IMAGENOIDCARD,
				ocr.FAILEDOPERATION_IMAGESIZETOOLARGE:
				return "", "", app.Invalid("无法识别身份证，请上传清晰完整的人像面")
			}
		}

		return "", "", &app.Error{
			Status:  http.StatusBadGateway,
			Code:    enum.ErrorUnavailable,
			Message: "身份证自动识别暂不可用，请重试",
		}
	}

	if response == nil || response.Response == nil || response.Response.Name == nil || response.Response.IdNum == nil ||
		*response.Response.Name == "" || *response.Response.IdNum == "" {
		return "", "", app.Invalid("未识别到姓名和身份证号，请上传清晰完整的人像面")
	}

	return *response.Response.Name, *response.Response.IdNum, nil
}
