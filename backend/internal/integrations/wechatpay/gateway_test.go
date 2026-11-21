package wechatpay

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

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
