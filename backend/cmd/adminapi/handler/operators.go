package handler

import (
	"net/http"
	"slices"
	"strings"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type operatorCatalog struct {
	identity.OperatorPermissionCatalog
}

func newOperatorCatalog(routes []httpx.Route) operatorCatalog {
	catalog := operatorCatalog{
		OperatorPermissionCatalog: identity.OperatorPermissionCatalog{
			Pages: []identity.OperatorPage{
				{
					Path:  "/tasks",
					Label: "营销任务",
				},
				{
					Path:  "/contents",
					Label: "内容池",
				},
				{
					Path:  "/users",
					Label: "分发员报名审核与管理",
				},
				{
					Path:  "/admin/realname",
					Label: "实名认证记录",
				},
				{
					Path:  "/reviews",
					Label: "执行与审核",
				},
				{
					Path:  "/settlements",
					Label: "分发结算",
				},
				{
					Path:  "/withdrawals",
					Label: "提现记录",
				},
				{
					Path:  "/ledger",
					Label: "余额流水",
				},
				{
					Path:  "/agreements",
					Label: "分发协议",
				},
				{
					Path:  "/feedback",
					Label: "反馈与咨询",
				},
				{
					Path:  "/audits",
					Label: "操作审计",
				},
			},
			Endpoints: make([]identity.OperatorEndpoint, 0, len(routes)),
		},
	}

	for _, route := range routes {
		catalog.Endpoints = append(catalog.Endpoints, identity.OperatorEndpoint{
			Key:   route.Method + " " + route.Path,
			Label: route.Label,
		})
	}

	return catalog
}

func (catalog operatorCatalog) authorizeEndpoint(ctx *gin.Context) {
	account := httpapi.CurrentPrincipal(ctx).Operator
	key := ctx.Request.Method + " " + strings.TrimPrefix(ctx.FullPath(), apiPrefix)
	if catalog.hasEndpoint(key) && (account.SuperAdmin || slices.Contains(account.Endpoints, key)) {
		ctx.Next()
		return
	}

	httpx.RespondError(ctx, app.Forbidden("当前账号无权访问此接口"))
	ctx.Abort()
}

func (catalog operatorCatalog) hasEndpoint(key string) bool {
	return slices.ContainsFunc(catalog.Endpoints, func(endpoint identity.OperatorEndpoint) bool {
		return endpoint.Key == key
	})
}

func (catalog operatorCatalog) validate(grants identity.OperatorGrants) error {
	if grants.Pages == nil || grants.Endpoints == nil {
		return app.Invalid("请明确指定页面和接口权限，未勾选时传空列表")
	}

	if len(grants.Pages) > len(catalog.Pages) || len(grants.Endpoints) > len(catalog.Endpoints) {
		return app.Invalid("权限列表无效")
	}

	for index, path := range grants.Pages {
		known := slices.ContainsFunc(catalog.Pages, func(page identity.OperatorPage) bool {
			return page.Path == path
		})
		if !known || slices.Contains(grants.Pages[:index], path) {
			return app.Invalid("页面权限无效或重复")
		}
	}

	for index, key := range grants.Endpoints {
		if !catalog.hasEndpoint(key) || slices.Contains(grants.Endpoints[:index], key) {
			return app.Invalid("接口权限无效或重复")
		}
	}

	return nil
}

func operatorProfile(ctx *gin.Context) (identity.OperatorDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")

	return httpapi.CurrentPrincipal(ctx).Operator, nil
}

func requireSuperAdmin(ctx *gin.Context) {
	if httpapi.CurrentPrincipal(ctx).Operator.SuperAdmin {
		ctx.Next()
		return
	}

	httpx.RespondError(ctx, app.Forbidden("仅超级管理员可管理运营账号"))
	ctx.Abort()
}

func (handler *Handler) registerOperatorManagement(protected *gin.RouterGroup, catalog operatorCatalog) {
	management := protected.Group("")
	management.Use(requireSuperAdmin)
	httpx.Register(management, []httpx.Route{
		{
			Method: http.MethodGet,
			Path:   "/operators/permissions",
			Handler: httpx.Adapt(func(ctx *gin.Context) (identity.OperatorPermissionCatalog, error) {
				return catalog.OperatorPermissionCatalog, nil
			}),
		},
		{
			Method: http.MethodGet,
			Path:   "/operators",
			Handler: httpx.Adapt(func(ctx *gin.Context) ([]identity.OperatorDTO, error) {
				return handler.services.Operators(ctx.Request.Context())
			}),
		},
		{
			Method: http.MethodPost,
			Path:   "/operators",
			Handler: httpx.JSON(func(ctx *gin.Context, input identity.CreateOperatorInput) (identity.OperatorDTO, error) {
				if err := catalog.validate(input.OperatorGrants); err != nil {
					return identity.OperatorDTO{}, err
				}

				return handler.services.CreateOperator(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
			}),
		},
		{
			Method: http.MethodPost,
			Path:   "/operators/:id",
			Handler: httpx.JSON(func(ctx *gin.Context, input identity.UpdateOperatorInput) (identity.OperatorDTO, error) {
				if err := catalog.validate(input.OperatorGrants); err != nil {
					return identity.OperatorDTO{}, err
				}

				return handler.services.UpdateOperator(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
			}),
		},
	}...)
}
