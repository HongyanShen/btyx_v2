package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/gin-gonic/gin"
)

func TestRoutingAndRecoveryErrorsKeepResponseEnvelope(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	router := minihandler.NewRouter(services)
	httpx.Register(router.Group(""), httpx.Route{
		Method: http.MethodGet,
		Path:   "/panic-test",
		Handler: httpx.Adapt(func(ctx *gin.Context) (*httpx.NoData, error) {
			panic("test internal error")
		}),
	})

	cases := []struct {
		method string
		path   string
		status int
	}{
		{
			method: http.MethodGet,
			path:   "/missing-route",
			status: http.StatusNotFound,
		},
		{
			method: http.MethodPost,
			path:   "/healthz",
			status: http.StatusMethodNotAllowed,
		},
		{
			method: http.MethodGet,
			path:   "/panic-test",
			status: http.StatusInternalServerError,
		},
	}
	for _, test := range cases {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			var envelope struct {
				Code int             `json:"code"`
				Msg  string          `json:"msg"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("non-JSON error response: %v", err)
			}

			if response.Code != test.status || envelope.Code != test.status || envelope.Msg == "" || string(envelope.Data) != "null" {
				t.Fatalf("invalid error response: status=%d, body=%s", response.Code, response.Body.String())
			}
		})
	}
}
