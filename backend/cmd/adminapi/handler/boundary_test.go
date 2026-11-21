package handler_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	adminhandler "baituber.cn/yanxuan/v2/cmd/adminapi/handler"
	minihandler "baituber.cn/yanxuan/v2/cmd/miniapi/handler"
	"baituber.cn/yanxuan/v2/internal/testkit"
	"github.com/gin-gonic/gin"
)

func TestProcessRoutersKeepTheirOwnCompleteRouteSurface(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	shared := []string{
		"GET /healthz",
		"GET /api/v1/assignments",
		"GET /api/v1/assignments/:id",
		"GET /api/v1/feedback",
		"GET /api/v1/files/:id",
		"GET /api/v1/files/:id/download",
		"GET /api/v1/ledger",
		"GET /api/v1/withdrawals",
		"POST /api/v1/files/uploads",
		"POST /api/v1/files/:id/complete",
	}
	cases := []struct {
		name   string
		router *gin.Engine
		routes []string
	}{
		{
			name:   "adminapi",
			router: adminhandler.NewRouter(services),
			routes: []string{
				"GET /api/v1/agreements",
				"GET /api/v1/audits",
				"GET /api/v1/identity-verifications",
				"GET /api/v1/identity-verifications/documents",
				"GET /api/v1/auth/me",
				"GET /api/v1/contents",
				"GET /api/v1/marketing-tags",
				"GET /api/v1/operators",
				"GET /api/v1/operators/permissions",
				"GET /api/v1/settlements",
				"GET /api/v1/settlements/preview",
				"GET /api/v1/tasks",
				"GET /api/v1/marketing-settings",
				"GET /api/v1/tasks/:id/versions",
				"GET /api/v1/users",
				"GET /api/v1/users/:id/enrollment-documents",
				"POST /api/v1/agreements",
				"POST /api/v1/assignments/:id/reissue",
				"POST /api/v1/assignments/:id/review",
				"POST /api/v1/assignments/:id/revoke",
				"POST /api/v1/auth/login",
				"POST /api/v1/contents",
				"POST /api/v1/contents/:id/tags",
				"POST /api/v1/feedback/:id/reply",
				"POST /api/v1/ledger/:id/adjust",
				"POST /api/v1/marketing-tags",
				"POST /api/v1/marketing-tags/:id/delete",
				"POST /api/v1/marketing-settings",
				"POST /api/v1/marketing-tags/:id/rename",
				"POST /api/v1/operators",
				"POST /api/v1/operators/:id",
				"POST /api/v1/settlements",
				"POST /api/v1/tasks",
				"POST /api/v1/tasks/:id/assign",
				"POST /api/v1/tasks/:id/versions",
				"POST /api/v1/tasks/:id/visibility",
				"POST /api/v1/users/:id/block",
				"POST /api/v1/users/:id/review",
			},
		},
		{
			name:   "miniapi",
			router: minihandler.NewRouter(services),
			routes: []string{
				"GET /api/v1/agreement",
				"GET /api/v1/me",
				"GET /api/v1/notifications",
				"GET /api/v1/tasks/today",
				"GET /api/v1/wallet",
				"POST /api/v1/agreement/consent",
				"POST /api/v1/assignments/:id/steps",
				"POST /api/v1/assignments/:id/submissions",
				"POST /api/v1/auth/local",
				"POST /api/v1/auth/logout",
				"POST /api/v1/auth/wechat",
				"POST /api/v1/enrollment",
				"POST /api/v1/feedback",
				"POST /api/v1/me/identity",
				"POST /api/v1/me/identity/recognize",
				"POST /api/v1/me/phone",
				"POST /api/v1/notifications/:id/read",
				"POST /api/v1/tasks/claim",
				"POST /api/v1/withdrawals",
				"POST /callbacks/wechatpay/transfer",
				"PUT /api/v1/me",
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			want := append(slices.Clone(shared), test.routes...)
			slices.Sort(want)
			var actual []string
			for _, route := range test.router.Routes() {
				actual = append(actual, route.Method+" "+route.Path)
				if route.Path == "/healthz" || route.Path == "/callbacks/wechatpay/transfer" || route.Path == "/api/v1/auth/login" || route.Path == "/api/v1/auth/wechat" || route.Path == "/api/v1/auth/local" {
					continue
				}

				response := httptest.NewRecorder()
				test.router.ServeHTTP(response, httptest.NewRequest(route.Method, route.Path, nil))
				if response.Code != http.StatusUnauthorized {
					t.Errorf("protected route lost authentication: %s %s status=%d", route.Method, route.Path, response.Code)
				}
			}

			slices.Sort(actual)
			if !slices.Equal(want, actual) {
				t.Fatalf("process routes changed:\nwant: %v\ngot: %v", want, actual)
			}
		})
	}
}

func TestMiniProductionRouterDoesNotExposeDemoLogin(t *testing.T) {
	services := testkit.Service(t, "../../../..")
	services.Config.Demo = false
	router := minihandler.NewRouter(services)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/local", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("production exposes demo login: %d", response.Code)
	}
}
