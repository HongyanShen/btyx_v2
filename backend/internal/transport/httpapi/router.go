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
)

type healthDTO struct {
	Ready bool `json:"ready"`
}

// NewRouter installs only the HTTP behavior shared by the API processes.
func NewRouter(runtime *app.Runtime) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	gin.EnableJsonDecoderDisallowUnknownFields()

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(requestMiddleware(), recoverMiddleware())
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
	return func(ctx *gin.Context) {
		principal, err := service.Authenticate(ctx.Request.Context(), BearerToken(ctx))
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

func requestMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestContext := logging.NewRequest(ctx.Request.Context())
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Header("X-Request-ID", logging.RequestID(requestContext))
		ctx.Header("X-Content-Type-Options", "nosniff")
		started := time.Now()
		logging.From(requestContext).Info().
			Str("method", ctx.Request.Method).
			Str("route", ctx.FullPath()).
			Msg("http request started")

		defer func() {
			logging.From(requestContext).Info().
				Str("method", ctx.Request.Method).
				Str("route", ctx.FullPath()).
				Int("status", ctx.Writer.Status()).
				Str("duration", time.Since(started).String()).
				Msg("http request completed")
		}()

		origin := ctx.GetHeader("Origin")
		if origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173" {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Vary", "Origin")
			ctx.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			ctx.Header("Access-Control-Expose-Headers", "X-Request-ID, X-Total-Count")
		}

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
				logging.From(ctx.Request.Context()).Error().
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
