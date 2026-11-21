package wechatmini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (exchange roundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return exchange(request)
}

func TestPhoneNumberExchange(t *testing.T) {
	for _, appID := range []string{"expected-app", "another-app"} {
		t.Run(appID, func(t *testing.T) {
			client := NewPhoneClient("expected-app", "test-secret")
			calls := 0
			client.client.Transport = roundTrip(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != http.MethodPost || request.URL.Host != "api.weixin.qq.com" {
					t.Fatal("unexpected WeChat request method or host")
				}

				var input map[string]string
				if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
					t.Fatal(err)
				}

				var jsonBody string
				switch request.URL.Path {
				case "/cgi-bin/stable_token":
					if input["appid"] != "expected-app" || input["secret"] != "test-secret" ||
						input["grant_type"] != "client_credential" || calls != 1 {
						t.Fatal("invalid stable token request")
					}

					jsonBody = `{"access_token":"test+token","expires_in":7200}`
				case "/wxa/business/getuserphonenumber":
					if request.URL.Query().Get("access_token") != "test+token" || input["code"] != "one-time-code" || calls != 2 {
						t.Fatal("invalid phone number request")
					}

					jsonBody = `{"errcode":0,"phone_info":{"phoneNumber":"13800000000","watermark":{"appid":"` + appID + `"}}}`
				default:
					t.Fatalf("unexpected WeChat path %q", request.URL.Path)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(jsonBody)),
				}, nil
			})

			number, err := client.Number(context.Background(), "one-time-code")
			if appID == "expected-app" {
				if err != nil || number != "13800000000" {
					t.Fatal("expected verified phone number")
				}
			} else if err == nil || number != "" {
				t.Fatal("accepted a phone number issued for a different app")
			}
		})
	}
}
