package handler

import (
	"baituber.cn/yanxuan/v2/internal/modules/creative"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (handler *Handler) creativeCatalog(ctx *gin.Context) (creative.Catalog, error) {
	return handler.services.Creative.Catalog(ctx.Request.Context(), httpapi.CurrentUser(ctx))
}

func (handler *Handler) creativeClaim(ctx *gin.Context) (string, error) {
	return handler.services.Creative.Claim(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), ctx.GetHeader("Idempotency-Key"))
}

func (handler *Handler) creativeSubmit(ctx *gin.Context, input creative.SubmitInput) (*httpx.NoData, error) {
	return nil, handler.services.Creative.Submit(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) creativeLinks(ctx *gin.Context, input creative.LinksInput) (*httpx.NoData, error) {
	return nil, handler.services.Creative.Links(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) creativeEvidence(ctx *gin.Context, input creative.Link) (*httpx.NoData, error) {
	return nil, handler.services.Creative.Evidence(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) creativeRoutes() []httpx.Route {
	return []httpx.Route{
		{
			Method:  http.MethodGet,
			Path:    "/creative/catalog",
			Label:   "",
			Handler: httpx.Adapt(handler.creativeCatalog),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/tasks/:id/claim",
			Label:   "",
			Handler: httpx.Adapt(handler.creativeClaim),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/works/:id/submit",
			Label:   "",
			Handler: httpx.JSON(handler.creativeSubmit),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/works/:id/links",
			Label:   "",
			Handler: httpx.JSON(handler.creativeLinks),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/works/:id/evidence",
			Label:   "",
			Handler: httpx.JSON(handler.creativeEvidence),
		},
	}
}
