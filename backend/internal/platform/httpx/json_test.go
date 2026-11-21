package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/gin-gonic/gin"
)

type typedRequest struct {
	Name     string        `json:"name" binding:"required"`
	Platform enum.Platform `json:"platform"`
}

type typedResponse struct {
	Name string `json:"name"`
}

func TestJSONBindsTypedRequestBeforeCallingHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gin.EnableJsonDecoderDisallowUnknownFields()
	called := 0
	router := gin.New()
	Register(router.Group(""), Route{
		Method: http.MethodPost,
		Path:   "/typed",
		Handler: JSON(func(ctx *gin.Context, input typedRequest) (typedResponse, error) {
			called++

			return typedResponse{Name: input.Name}, nil
		}),
	})
	cases := []struct {
		body   string
		status int
	}{
		{
			body:   `{"name":"test","platform":"dy"}`,
			status: http.StatusOK,
		},
		{
			body:   `{"name":`,
			status: http.StatusBadRequest,
		},
		{
			body:   `{"name":123,"platform":"dy"}`,
			status: http.StatusBadRequest,
		},
		{
			body:   `{"name":"test","platform":"unknown"}`,
			status: http.StatusBadRequest,
		},
		{
			body:   `{"name":"test","platform":"dy","extra":"field"}`,
			status: http.StatusBadRequest,
		},
		{
			body:   `{"platform":"dy"}`,
			status: http.StatusBadRequest,
		},
	}
	for _, test := range cases {
		t.Run(test.body, func(t *testing.T) {
			before := called
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/typed", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			var envelope struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}

			if response.Code != test.status {
				t.Fatalf("wrong status: %d %s", response.Code, response.Body.String())
			}

			if test.status == http.StatusOK {
				if called != before+1 || string(envelope.Data) != `{"name":"test"}` {
					t.Fatalf("typed handler did not receive decoded input: %s", response.Body.String())
				}
			} else if called != before || envelope.Code != test.status || string(envelope.Data) != "null" {
				t.Fatalf("invalid input reached handler or lost error envelope: %s", response.Body.String())
			}
		})
	}
}

func TestTypedResponsesKeepNullDataAndHideFailedResults(t *testing.T) {
	router := gin.New()
	Register(router.Group(""),
		Route{
			Method: http.MethodPost,
			Path:   "/empty",
			Handler: Adapt(func(ctx *gin.Context) (*NoData, error) {
				return nil, nil
			}),
		},
		Route{
			Method: http.MethodGet,
			Path:   "/failed",
			Handler: Adapt(func(ctx *gin.Context) (typedResponse, error) {
				return typedResponse{Name: "must-not-be-exposed"}, app.Conflict("记录已更新")
			}),
		},
	)
	for path, status := range map[string]int{
		"/empty":  http.StatusOK,
		"/failed": http.StatusConflict,
	} {
		method := http.MethodGet
		if path == "/empty" {
			method = http.MethodPost
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != status || !strings.Contains(response.Body.String(), `"data":null`) || strings.Contains(response.Body.String(), "must-not-be-exposed") {
			t.Fatalf("typed response changed protocol: %d %s", response.Code, response.Body.String())
		}
	}
}
