package httpx

import (
	"errors"
	"net/http"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
)

// Handler keeps the response DTO concrete until the final HTTP renderer.
type Handler[T any] func(*gin.Context) (T, error)

type Route struct {
	Method  string
	Path    string
	Label   string
	Handler gin.HandlerFunc
}

// A nil *NoData represents a successful JSON response with data: null.
type NoData struct{}

type FileResponse struct {
	Path      string
	Name      string
	MediaType string
}

type EmptyResponse struct {
	Status int
}

// StatusError preserves protocols that require an empty error response body.
type StatusError struct {
	Status int
}

func (err *StatusError) Error() string {
	return http.StatusText(err.Status)
}

func Register(group *gin.RouterGroup, routes ...Route) {
	for _, route := range routes {
		group.Handle(route.Method, route.Path, route.Handler)
	}
}

func Adapt[T any](handler Handler[T]) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		data, err := handler(ctx)
		if err != nil {
			var statusError *StatusError
			if errors.As(err, &statusError) {
				logging.From(ctx.Request.Context()).Warn().
					Int("status", statusError.Status).
					Msg("protocol request rejected")

				ctx.Status(statusError.Status)

				return
			}

			RespondError(ctx, err)

			return
		}

		// Only the final renderer erases the response type for protocol dispatch.
		switch response := any(data).(type) {
		case FileResponse:
			ctx.Header("Content-Type", response.MediaType)
			ctx.Header("Cache-Control", "private, no-store")
			ctx.FileAttachment(response.Path, response.Name)
		case EmptyResponse:
			ctx.Status(response.Status)
		default:
			Respond(ctx, data)
		}
	}
}

// JSON binds the request before calling a handler with concrete input/output types.
func JSON[Input, Output any](handler func(*gin.Context, Input) (Output, error)) gin.HandlerFunc {
	return Adapt(func(ctx *gin.Context) (Output, error) {
		var input Input
		if err := ctx.ShouldBindJSON(&input); err != nil {
			var zero Output

			return zero, app.Invalid("请求字段、枚举值或格式无效")
		}

		return handler(ctx, input)
	})
}
