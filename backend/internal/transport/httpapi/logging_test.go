package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

func TestHTTPLogsShareServerRequestIDOnSuccessFailureAndPanic(t *testing.T) {
	for _, outcome := range []string{"success", "error", "panic"} {
		t.Run(outcome, func(t *testing.T) {
			var output bytes.Buffer
			root := logging.WithLogger(context.Background(), logging.New(&output, "testapi"))
			router := NewRouter(nil)
			httpx.Register(router.Group(""), httpx.Route{
				Method: http.MethodPost,
				Path:   "/business",
				Handler: httpx.Adapt(func(ctx *gin.Context) (*httpx.NoData, error) {
					logging.From(ctx.Request.Context()).Info().Msg("business action")
					if outcome == "panic" {
						panic("secret-panic-password")
					}

					if outcome == "error" {
						return nil, errors.New("secret-error-token")
					}

					return nil, nil
				}),
			})
			request := httptest.NewRequest(http.MethodPost, "/business?token=secret-query-token", strings.NewReader("secret-request-body"))
			request = request.WithContext(root)
			request.Header.Set("Authorization", "Bearer secret-authorization-token")
			request.Header.Set("X-Request-ID", "untrusted-client-id")
			request.Header.Set("Origin", "http://127.0.0.1:5173")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			requestID := response.Header().Get("X-Request-ID")
			if _, err := ksuid.Parse(requestID); err != nil || requestID == "untrusted-client-id" {
				t.Fatalf("missing server-issued request ID: %q", requestID)
			}

			wantStatus := http.StatusOK
			if outcome != "success" {
				wantStatus = http.StatusInternalServerError
			}

			if response.Code != wantStatus || !strings.Contains(response.Header().Get("Access-Control-Expose-Headers"), "X-Request-ID") {
				t.Fatalf("incorrect response status or invisible request ID: %d", response.Code)
			}

			startedCount := 0
			completedCount := 0
			var messages []string
			for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				var entry struct {
					RequestID string `json:"request_id"`
					Message   string `json:"message"`
					Status    int    `json:"status"`
					Method    string `json:"method"`
					Route     string `json:"route"`
					Duration  string `json:"duration"`
				}
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatalf("non-JSON log: %q", line)
				}

				if entry.RequestID != requestID || strings.Contains(line, "secret-") || strings.Contains(line, "untrusted-client-id") {
					t.Fatalf("unsafe or uncorrelated log: %s", line)
				}

				messages = append(messages, entry.Message)
				if entry.Message == "http request started" {
					startedCount++
					if entry.Method != http.MethodPost || entry.Route != "/business" || entry.Status != 0 || entry.Duration != "" {
						t.Fatalf("incorrect start log: %s", line)
					}
				}

				if entry.Message == "http request completed" {
					completedCount++
					duration, err := time.ParseDuration(entry.Duration)
					if entry.Status != wantStatus || entry.Method != http.MethodPost || entry.Route != "/business" || err != nil || duration < 0 {
						t.Fatalf("incorrect completion log: %s", line)
					}
				}
			}

			if startedCount != 1 || completedCount != 1 || messages[0] != "http request started" || messages[len(messages)-1] != "http request completed" {
				t.Fatalf("request must start and complete exactly once, including after panic: %s", output.String())
			}
		})
	}
}

func TestHTTPStartIsLoggedBeforeHandlerRuns(t *testing.T) {
	var output bytes.Buffer
	root := logging.WithLogger(context.Background(), logging.New(&output, "testapi"))
	router := NewRouter(nil)
	router.GET("/slow", func(ctx *gin.Context) {
		var entry struct {
			RequestID string `json:"request_id"`
			Message   string `json:"message"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil {
			t.Fatalf("handler must already have exactly one start log: %v", err)
		}

		if entry.Message != "http request started" || entry.RequestID != logging.RequestID(ctx.Request.Context()) {
			t.Fatalf("missing start log while request is still running: %s", output.String())
		}

		ctx.Status(http.StatusAccepted)
	})
	request := httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(root)
	router.ServeHTTP(httptest.NewRecorder(), request)
}

func TestHTTPBoundaryRequestsLogStartAndCompletion(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{
			name:   "not found",
			method: http.MethodGet,
			path:   "/unknown?token=secret-query-token",
			status: http.StatusNotFound,
		},
		{
			name:   "method not allowed",
			method: http.MethodGet,
			path:   "/business",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:   "preflight",
			method: http.MethodOptions,
			path:   "/business",
			status: http.StatusNoContent,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			root := logging.WithLogger(context.Background(), logging.New(&output, "testapi"))
			router := NewRouter(nil)
			router.POST("/business", func(ctx *gin.Context) {
				t.Fatal("boundary request unexpectedly reached handler")
			})
			request := httptest.NewRequest(test.method, test.path, nil).WithContext(root)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || strings.Contains(output.String(), "secret-") {
				t.Fatalf("incorrect or unsafe response: status=%d logs=%s", response.Code, output.String())
			}

			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			if len(lines) != 2 {
				t.Fatalf("boundary request must have two lifecycle logs: %s", output.String())
			}

			for index, line := range lines {
				var entry struct {
					RequestID string `json:"request_id"`
					Message   string `json:"message"`
					Status    int    `json:"status"`
				}
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatal(err)
				}

				if entry.RequestID != response.Header().Get("X-Request-ID") {
					t.Fatalf("boundary logs lost correlation: %s", line)
				}

				if index == 0 && entry.Message != "http request started" || index == 1 && (entry.Message != "http request completed" || entry.Status != test.status) {
					t.Fatalf("incorrect boundary lifecycle log: %s", line)
				}
			}
		})
	}
}
