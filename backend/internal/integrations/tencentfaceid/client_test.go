package tencentfaceid

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	faceid "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/faceid/v20180301"
)

type verifyRequestFunc func(context.Context, *faceid.IdCardOCRVerificationRequest) (*faceid.IdCardOCRVerificationResponse, error)

func (verify verifyRequestFunc) IdCardOCRVerificationWithContext(ctx context.Context, request *faceid.IdCardOCRVerificationRequest) (*faceid.IdCardOCRVerificationResponse, error) {
	return verify(ctx, request)
}

func TestCheckUsesFinalFieldsAndRequiresExplicitMatch(t *testing.T) {
	cases := []struct {
		result string
		status int
	}{
		{result: "0"},
		{
			result: "-1",
			status: http.StatusBadRequest,
		},
		{
			result: "-2",
			status: http.StatusBadRequest,
		},
		{
			result: "-3",
			status: http.StatusBadRequest,
		},
		{
			result: "-4",
			status: http.StatusServiceUnavailable,
		},
		{
			result: "-5",
			status: http.StatusBadRequest,
		},
		{
			result: "-6",
			status: http.StatusServiceUnavailable,
		},
		{
			result: "-7",
			status: http.StatusServiceUnavailable,
		},
		{
			result: "unknown",
			status: http.StatusServiceUnavailable,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.result, func(t *testing.T) {
			client := &Client{api: verifyRequestFunc(func(_ context.Context, request *faceid.IdCardOCRVerificationRequest) (*faceid.IdCardOCRVerificationResponse, error) {
				if request.Name == nil || *request.Name != "更正姓名" || request.IdCard == nil ||
					*request.IdCard != "990000199001010017" || request.ImageUrl != nil || request.ImageBase64 != nil {
					t.Fatal("expected the final name and number pair")
				}

				return &faceid.IdCardOCRVerificationResponse{
					Response: &faceid.IdCardOCRVerificationResponseParams{
						Result:      common.StringPtr(testCase.result),
						Description: common.StringPtr("private provider details"),
					},
				}, nil
			})}

			err := client.Check(context.Background(), "更正姓名", "990000199001010017")
			if testCase.status == 0 {
				if err != nil {
					t.Fatal("positive match must be accepted")
				}

				return
			}

			var failure *app.Error
			if !errors.As(err, &failure) || failure.Status != testCase.status ||
				strings.Contains(failure.Message, "private provider details") {
				t.Fatal("unexpected public verification error")
			}
		})
	}
}
