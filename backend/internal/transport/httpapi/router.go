package httpapi

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type healthDTO struct {
	Ready bool `json:"ready"`
}

// NewRouter installs only the HTTP behavior shared by the API processes.
func NewRouter(runtime *app.Runtime, platform enum.AuditPlatform) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	gin.EnableJsonDecoderDisallowUnknownFields()

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(requestMiddleware(platform), auditHTTPResponses(runtime), recoverMiddleware(), captureAuditRequest)
	_ = router.SetTrustedProxies(nil)
	router.NoRoute(func(ctx *gin.Context) {
		httpx.Abort(ctx, http.StatusNotFound, "接口不存在")
	})
	router.NoMethod(func(ctx *gin.Context) {
		httpx.Abort(ctx, http.StatusMethodNotAllowed, "请求方法不支持")
	})

	httpx.Register(router.Group(""), httpx.Route{
		Method: http.MethodGet,
		Path:   "/healthz",
		Handler: httpx.Adapt(func(ctx *gin.Context) (healthDTO, error) {
			err := runtime.DB.PingContext(ctx.Request.Context())

			return healthDTO{Ready: err == nil}, err
		}),
	})

	return router
}

func Authenticate(service *identity.Service, role enum.Role) gin.HandlerFunc {
	return AuthenticateWithToken(service, role, BearerToken)
}

func AuthenticateWithToken(service *identity.Service, role enum.Role, token func(*gin.Context) string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, err := service.Authenticate(ctx.Request.Context(), token(ctx))
		if err != nil {
			httpx.RespondError(ctx, err)
			ctx.Abort()
			return
		}

		if principal.Role != role {
			httpx.RespondError(ctx, app.Forbidden("当前身份无权访问此接口"))
			ctx.Abort()
			return
		}

		ctx.Set("principal", principal)
		ctx.Next()
	}
}

func BearerToken(ctx *gin.Context) string {
	return strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer ")
}

func CurrentUser(ctx *gin.Context) string {
	return CurrentPrincipal(ctx).ID
}

func CurrentPrincipal(ctx *gin.Context) identity.Principal {
	return ctx.MustGet("principal").(identity.Principal)
}

func requestMiddleware(platform enum.AuditPlatform) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestContext := logging.NewRequest(ctx.Request.Context())
		route := ctx.FullPath()
		name := ctx.Request.Method + " " + route
		if route != "" {
			requestContext = app.WithAuditOrigin(requestContext, platform, name)
		}

		requestContext, span := otel.Tracer("yanxuan/http").Start(requestContext, name, trace.WithSpanKind(trace.SpanKindServer))
		span.SetAttributes(attribute.String("http.route", route), attribute.String("request_id", logging.RequestID(requestContext)))
		defer func() {
			status := ctx.Writer.Status()
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if status >= 500 {
				span.SetStatus(codes.Error, "")
			}

			span.End()
		}()
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Header("X-Request-ID", logging.RequestID(requestContext))
		ctx.Header("X-Content-Type-Options", "nosniff")
		started := time.Now()
		logging.Info(requestContext).
			Str("method", ctx.Request.Method).
			Str("route", ctx.FullPath()).
			Msg("http request started")

		defer func() {
			logging.Info(requestContext).
				Str("method", ctx.Request.Method).
				Str("route", ctx.FullPath()).
				Int("status", ctx.Writer.Status()).
				Str("duration", time.Since(started).String()).
				Msg("http request completed")
		}()

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)

			return
		}

		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20)
		ctx.Next()
	}
}

func recoverMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logging.Error(ctx.Request.Context(), nil).
					Str("panic_type", fmt.Sprintf("%T", recovered)).
					Str("stack", string(debug.Stack())).
					Msg("http handler panic")
				if ctx.Writer.Written() {
					ctx.Abort()

					return
				}

				httpx.Abort(ctx, http.StatusInternalServerError, "服务暂时不可用，请稍后重试")
			}
		}()
		ctx.Next()
	}
}
