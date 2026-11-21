package handler

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/integrations/wechatpay"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	services *business.Service
	payments *wechatpay.Gateway
}

func NewRouter(services *business.Service, payments ...*wechatpay.Gateway) *gin.Engine {
	router := httpapi.NewRouter(services.Runtime)
	handler := &Handler{
		services: services,
	}

	if len(payments) > 0 {
		handler.payments = payments[0]
	}

	httpx.Register(router.Group(""), httpx.Route{
		Method:  http.MethodPost,
		Path:    "/callbacks/wechatpay/transfer",
		Handler: httpx.Adapt(handler.transferCallback),
	})

	public := router.Group("/api/v1")
	httpx.Register(public, []httpx.Route{
		{
			Method:  http.MethodPost,
			Path:    "/auth/wechat",
			Handler: httpx.JSON(handler.loginWechat),
		},
	}...)

	if services.Config.Demo {
		httpx.Register(public, httpx.Route{
			Method:  http.MethodPost,
			Path:    "/auth/local",
			Handler: httpx.JSON(handler.loginLocal),
		})
	}

	protected := public.Group("")
	protected.Use(httpapi.Authenticate(services.IdentityService, enum.RoleUser))
	httpx.Register(protected, httpapi.FileRoutes(services.FilesService, enum.RoleUser)...)
	httpx.Register(protected, httpapi.AssignmentRoutes(services.TaskService)...)
	httpx.Register(protected, httpapi.WalletRoutes(services.WalletService)...)
	httpx.Register(protected, httpapi.FeedbackRoutes(services.FeedbackService)...)
	httpx.Register(protected, []httpx.Route{
		{
			Method:  http.MethodPost,
			Path:    "/auth/logout",
			Handler: httpx.Adapt(handler.logout),
		},
		{
			Method:  http.MethodGet,
			Path:    "/me",
			Handler: httpx.Adapt(handler.profile),
		},
		{
			Method:  http.MethodPut,
			Path:    "/me",
			Handler: httpx.JSON(handler.updateProfile),
		},
		{
			Method:  http.MethodPost,
			Path:    "/me/phone",
			Handler: httpx.JSON(handler.verifyPhone),
		},
		{
			Method:  http.MethodPost,
			Path:    "/me/identity/recognize",
			Handler: httpx.JSON(handler.recognizeIdentity),
		},
		{
			Method:  http.MethodPost,
			Path:    "/me/identity",
			Handler: httpx.JSON(handler.verifyIdentity),
		},
		{
			Method:  http.MethodPost,
			Path:    "/enrollment",
			Handler: httpx.JSON(handler.applyEnrollment),
		},
		{
			Method:  http.MethodGet,
			Path:    "/agreement",
			Handler: httpx.Adapt(handler.currentAgreement),
		},
		{
			Method:  http.MethodPost,
			Path:    "/agreement/consent",
			Handler: httpx.JSON(handler.consentAgreement),
		},
		{
			Method:  http.MethodGet,
			Path:    "/tasks/today",
			Handler: httpx.Adapt(handler.todayAvailability),
		},
		{
			Method:  http.MethodPost,
			Path:    "/tasks/claim",
			Handler: httpx.Adapt(handler.claimTask),
		},
		{
			Method:  http.MethodPost,
			Path:    "/assignments/:id/steps",
			Handler: httpx.JSON(handler.completeStep),
		},
		{
			Method:  http.MethodPost,
			Path:    "/assignments/:id/submissions",
			Handler: httpx.JSON(handler.submit),
		},
		{
			Method:  http.MethodGet,
			Path:    "/wallet",
			Handler: httpx.Adapt(handler.wallet),
		},
		{
			Method:  http.MethodPost,
			Path:    "/withdrawals",
			Handler: httpx.JSON(handler.withdraw),
		},
		{
			Method:  http.MethodPost,
			Path:    "/feedback",
			Handler: httpx.JSON(handler.createFeedback),
		},
		{
			Method:  http.MethodGet,
			Path:    "/notifications",
			Handler: httpx.Adapt(handler.notifications),
		},
		{
			Method:  http.MethodPost,
			Path:    "/notifications/:id/read",
			Handler: httpx.Adapt(handler.readNotification),
		},
	}...)

	return router
}
