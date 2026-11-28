package operations

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	services *business.Service
}

const apiPrefix = "/api/v1"

func Register(router *gin.Engine, services *business.Service, authenticate gin.HandlerFunc, pages []identity.OperatorPage) OperatorCatalog {
	handler := &Handler{
		services: services,
	}

	routes := handler.businessRoutes()
	routes = append(routes, handler.creativeRoutes()...)
	routes = append(routes, httpapi.FileRoutes(services.FilesService, enum.RoleAdmin)...)
	routes = append(routes, httpapi.AssignmentRoutes(services.TaskService)...)
	routes = append(routes, httpapi.WalletRoutes(services.WalletService)...)
	routes = append(routes, httpapi.FeedbackRoutes(services.FeedbackService)...)

	catalog := newOperatorCatalog(pages, routes)
	protected := router.Group(apiPrefix, authenticate, func(ctx *gin.Context) {
		principal := httpapi.CurrentPrincipal(ctx)
		principal.Operator = catalog.EffectiveGrants(principal.Operator)
		ctx.Set("principal", principal)
	})
	httpx.Register(protected, httpx.Route{
		Method:  http.MethodGet,
		Path:    "/auth/me",
		Handler: httpx.Adapt(operatorProfile),
	})
	operations := protected.Group("")
	operations.Use(catalog.authorizeEndpoint)
	httpx.Register(operations, routes...)
	httpx.Register(protected,
		httpx.Route{
			Method:  http.MethodGet,
			Path:    "/settlements/weeks",
			Handler: httpx.Adapt(handler.settlementWeeks),
		},
		httpx.Route{
			Method:  http.MethodGet,
			Path:    "/settlements/export",
			Handler: httpx.Adapt(handler.exportSettlement),
		},
	)
	handler.registerOperatorManagement(protected, catalog)
	return catalog
}

func (handler *Handler) businessRoutes() []httpx.Route {
	return []httpx.Route{
		{
			Method:  http.MethodGet,
			Path:    "/identity-verifications",
			Label:   "查看实名认证记录",
			Handler: httpx.Adapt(handler.identityRecords),
		},
		{
			Method:  http.MethodGet,
			Path:    "/identity-verifications/documents",
			Label:   "查看实名资料及证件照片",
			Handler: httpx.Adapt(handler.identityRecordsWithDocuments),
		},
		{
			Method:  http.MethodGet,
			Path:    "/users",
			Label:   "查看分发员与报名",
			Handler: httpx.Adapt(handler.users),
		},
		{
			Method:  http.MethodGet,
			Path:    "/users/:id/enrollment-documents",
			Label:   "查看报名实名资料及证件照片",
			Handler: httpx.Adapt(handler.enrollmentDocuments),
		},
		{
			Method:  http.MethodPost,
			Path:    "/users/:id/review",
			Label:   "复核分发员报名与编辑画像",
			Handler: httpx.JSON(handler.reviewEnrollment),
		},
		{
			Method:  http.MethodPost,
			Path:    "/users/:id/block",
			Label:   "调整分发员状态",
			Handler: httpx.JSON(handler.blockUser),
		},
		{
			Method:  http.MethodGet,
			Path:    "/marketing-tags",
			Label:   "查看营销标签库",
			Handler: httpx.Adapt(handler.marketingTags),
		},
		{
			Method:  http.MethodPost,
			Path:    "/marketing-tags",
			Label:   "添加营销标签",
			Handler: httpx.JSON(handler.createMarketingTag),
		},
		{
			Method:  http.MethodPost,
			Path:    "/marketing-tags/:id/rename",
			Label:   "重命名营销标签",
			Handler: httpx.JSON(handler.renameMarketingTag),
		},
		{
			Method:  http.MethodPost,
			Path:    "/marketing-tags/:id/delete",
			Label:   "删除空闲营销标签",
			Handler: httpx.JSON(handler.deleteMarketingTag),
		},
		{
			Method:  http.MethodGet,
			Path:    "/tasks",
			Label:   "查看营销任务",
			Handler: httpx.Adapt(handler.tasks),
		},
		{
			Method:  http.MethodGet,
			Path:    "/marketing-settings",
			Label:   "查看营销分发设置",
			Handler: httpx.Adapt(handler.marketingSettings),
		},
		{
			Method:  http.MethodPost,
			Path:    "/marketing-settings",
			Label:   "修改营销分发设置",
			Handler: httpx.JSON(handler.updateMarketingSettings),
		},
		{
			Method:  http.MethodPost,
			Path:    "/tasks",
			Label:   "创建营销任务",
			Handler: httpx.JSON(handler.createTask),
		},
		{
			Method:  http.MethodGet,
			Path:    "/tasks/:id/versions",
			Label:   "查看任务版本",
			Handler: httpx.Adapt(handler.versions),
		},
		{
			Method:  http.MethodPost,
			Path:    "/tasks/:id/versions",
			Label:   "发布任务版本",
			Handler: httpx.JSON(handler.publishVersion),
		},
		{
			Method:  http.MethodPost,
			Path:    "/tasks/:id/visibility",
			Label:   "调整任务上线状态",
			Handler: httpx.JSON(handler.setTaskVisibility),
		},
		{
			Method:  http.MethodPost,
			Path:    "/tasks/:id/assign",
			Label:   "指派任务",
			Handler: httpx.JSON(handler.assignTask),
		},
		{
			Method:  http.MethodPost,
			Path:    "/assignments/:id/revoke",
			Label:   "撤回派发",
			Handler: httpx.JSON(handler.revokeAssignment),
		},
		{
			Method:  http.MethodPost,
			Path:    "/assignments/:id/review",
			Label:   "审核平台提交",
			Handler: httpx.JSON(handler.reviewSubmission),
		},
		{
			Method:  http.MethodPost,
			Path:    "/assignments/:id/reissue",
			Label:   "重派任务",
			Handler: httpx.JSON(handler.reissueAssignment),
		},
		{
			Method:  http.MethodGet,
			Path:    "/settlements/preview",
			Label:   "预览结算",
			Handler: httpx.Adapt(handler.previewSettlement),
		},
		{
			Method:  http.MethodPost,
			Path:    "/settlements",
			Label:   "确认结算入账",
			Handler: httpx.JSON(handler.settle),
		},
		{
			Method:  http.MethodGet,
			Path:    "/settlements",
			Label:   "查看结算批次",
			Handler: httpx.Adapt(handler.settlements),
		},
		{
			Method:  http.MethodGet,
			Path:    "/distribution/settings",
			Label:   "查看分发设置",
			Handler: httpx.Adapt(handler.distributionSettings),
		},
		{
			Method:  http.MethodPost,
			Path:    "/distribution/settings",
			Label:   "修改分发设置",
			Handler: httpx.JSON(handler.updateDistributionSettings),
		},
		{
			Method:  http.MethodPost,
			Path:    "/ledger/:id/adjust",
			Label:   "调整余额流水",
			Handler: httpx.JSON(handler.adjustLedger),
		},
		{
			Method:  http.MethodGet,
			Path:    "/contents",
			Label:   "查看内容池",
			Handler: httpx.Adapt(handler.contents),
		},
		{
			Method:  http.MethodPost,
			Path:    "/contents",
			Label:   "新建内容",
			Handler: httpx.JSON(handler.createContent),
		},
		{
			Method:  http.MethodPost,
			Path:    "/contents/:id/tags",
			Label:   "修改内容池标签",
			Handler: httpx.JSON(handler.updateContentTags),
		},
		{
			Method:  http.MethodGet,
			Path:    "/audits",
			Label:   "查看操作审计",
			Handler: httpx.Adapt(handler.audits),
		},
		{
			Method:  http.MethodPost,
			Path:    "/feedback/:id/reply",
			Label:   "回复反馈",
			Handler: httpx.JSON(handler.replyFeedback),
		},
		{
			Method:  http.MethodGet,
			Path:    "/agreements",
			Label:   "查看分发协议",
			Handler: httpx.Adapt(handler.agreements),
		},
		{
			Method:  http.MethodPost,
			Path:    "/agreements",
			Label:   "发布分发协议",
			Handler: httpx.JSON(handler.publishAgreement),
		},
	}
}
