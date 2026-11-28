package wechatmini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
)

func TestSendSubscriptionCard(t *testing.T) {
	c := NewPhoneClient("test-app", "test-secret")
	c.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		response := "{\"access_token\":\"test+token\"}"
		if r.URL.Path != "/cgi-bin/stable_token" {
			if r.URL.Host != "api.weixin.qq.com" || r.URL.Path != "/cgi-bin/message/subscribe/send" || r.URL.Query().Get("access_token") != "test+token" {
				t.Fatal("incorrect send endpoint")
			}

			var payload struct {
				To string `json:"touser"`
				wxnotice.Card
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}

			if payload.To != "test-open-id" || payload.TemplateID != wxnotice.Settlement || payload.Data["amount6"].Value != "123.45元" || payload.Page != "pages/wallet/index" {
				t.Fatal("incorrect settlement payload")
			}

			response = "{\"errcode\":43101}"
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(response)),
		}, nil
	})
	code, err := c.SendCard(context.Background(), "test-open-id", wxnotice.Credit("123.45"))
	if err != nil || code != 43101 {
		t.Fatalf("unexpected WeChat outcome: %d %v", code, err)
	}
}
