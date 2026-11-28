package wechatpay

import (
	"bytes"
	"context"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

func TestTransferSubmitErrorLogsSafeProviderDiagnostics(t *testing.T) {
	var buffer bytes.Buffer
	ctx := logging.WithLogger(context.Background(), logging.New(&buffer, "test"))
	logSubmitError(ctx, &core.APIError{
		StatusCode: 400,
		Code:       "INVALID_REQUEST",
		Message:    "此IP地址不允许调用接口，包含模拟隐私字段",
		Body:       "模拟原始响应正文",
	})

	entry := buffer.String()
	for _, expected := range []string{`"wechat_http_status":400`, `"wechat_error_code":"INVALID_REQUEST"`, `"wechat_rejection":"ip_not_allowed"`} {
		if !strings.Contains(entry, expected) {
			t.Fatalf("missing safe field %q: %s", expected, entry)
		}
	}

	if strings.Contains(entry, "模拟隐私字段") || strings.Contains(entry, "模拟原始响应正文") {
		t.Fatal("third-party message or raw body was logged")
	}
}

func TestUnknownProviderStateDoesNotBecomeSuccess(t *testing.T) {
	if _, err := mapState(enum.WechatTransferState("NEW_UNKNOWN_STATE")); err == nil {
		t.Fatal("unknown payment state was accepted")
	}
}

func TestLocalDemoIdentityCannotReceiveRealTransfers(t *testing.T) {
	gateway := &Gateway{}
	_, err := gateway.Submit(context.Background(), transfer.Request{

		ID:        "local-transfer",
		OpenID:    "local:devtools",
		AmountFen: "1000",
	})
	if err == nil {
		t.Fatal("local demo identity reached a real payment channel")
	}
}

func TestUnsignedNotificationIsRejectedByOfficialSDK(t *testing.T) {
	handler, err := notify.NewRSANotifyHandler(
		strings.Repeat("a", 32),
		verifiers.NewSHA256WithRSAPubkeyVerifier("TEST_KEY", rsa.PublicKey{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	gateway := &Gateway{notifications: handler}
	request := httptest.NewRequest(http.MethodPost, "/callbacks/wechatpay", strings.NewReader(`{"state":"SUCCESS"}`))
	if _, err := gateway.ParseNotification(context.Background(), request); err == nil {
		t.Fatal("unsigned financial notification accepted")
	}
}
