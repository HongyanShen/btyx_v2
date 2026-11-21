package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/gin-gonic/gin"
)

type echoInput struct {
	Name string `json:"name"`
}

func TestRegisteredHandlersShareResponseAndDecodeErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/api"),
		Route{
			Method: http.MethodPost,
			Path:   "/echo",
			Handler: JSON(func(ctx *gin.Context, input echoInput) (echoInput, error) {
				return input, nil
			}),
		},
		Route{
			Method: http.MethodGet,
			Path:   "/conflict",
			Handler: Adapt(func(ctx *gin.Context) (*NoData, error) {
				return nil, app.Conflict("原记录已更新")
			}),
		},
		Route{
			Method: http.MethodGet,
			Path:   "/failure",
			Handler: Adapt(func(ctx *gin.Context) (*NoData, error) {
				return nil, errors.New("internal-sensitive-error")
			}),
		},
	)
	cases := []struct {
		method string
		path   string
		body   string
		status int
		text   string
	}{
		{
			method: http.MethodPost,
			path:   "/api/echo",
			body:   `{"name":"test"}`,
			status: http.StatusOK,
			text:   `"data":{"name":"test"}`,
		},
		{
			method: http.MethodPost,
			path:   "/api/echo",
			body:   `{"name":`,
			status: http.StatusBadRequest,
			text:   `"data":null`,
		},
		{
			method: http.MethodGet,
			path:   "/api/conflict",
			status: http.StatusConflict,
			text:   "原记录已更新",
		},
		{
			method: http.MethodGet,
			path:   "/api/failure",
			status: http.StatusInternalServerError,
			text:   "服务暂时不可用",
		},
	}
	for _, test := range cases {
		t.Run(test.path+test.body, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.text) || strings.Contains(body, "internal-sensitive-error") {
				t.Fatalf("unexpected handler response: status=%d body=%s", response.Code, body)
			}
		})
	}
}

func TestHandlerAdapterPreservesFileAndCallbackProtocols(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "attachment.txt")
	if err := os.WriteFile(filePath, []byte("attachment-body"), 0600); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	Register(router.Group(""),
		Route{
			Method: http.MethodGet,
			Path:   "/file",
			Handler: Adapt(func(ctx *gin.Context) (FileResponse, error) {
				return FileResponse{
					Path:      filePath,
					Name:      "attachment.txt",
					MediaType: "text/plain",
				}, nil
			}),
		},
		Route{
			Method: http.MethodPost,
			Path:   "/callback",
			Handler: Adapt(func(ctx *gin.Context) (EmptyResponse, error) {
				return EmptyResponse{Status: http.StatusNoContent}, nil
			}),
		},
		Route{
			Method: http.MethodPost,
			Path:   "/bad-callback",
			Handler: Adapt(func(ctx *gin.Context) (*NoData, error) {
				return nil, &StatusError{Status: http.StatusUnauthorized}
			}),
		},
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/file", nil))
	if response.Code != http.StatusOK || response.Body.String() != "attachment-body" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("file response was wrapped or cached: %d %s", response.Code, response.Body.String())
	}

	for path, status := range map[string]int{
		"/callback":     http.StatusNoContent,
		"/bad-callback": http.StatusUnauthorized,
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != status || response.Body.Len() != 0 {
			t.Fatalf("callback protocol changed: %d %s", response.Code, response.Body.String())
		}
	}
}
