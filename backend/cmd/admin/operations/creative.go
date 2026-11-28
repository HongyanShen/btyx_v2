package operations

import (
	"baituber.cn/yanxuan/v2/internal/modules/creative"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (handler *Handler) creativeTasks(ctx *gin.Context) ([]creative.TaskDTO, error) {
	return handler.services.Creative.Tasks(ctx.Request.Context(), false)
}

func (handler *Handler) creativeCreate(ctx *gin.Context, input creative.TaskInput) (string, error) {
	return handler.services.Creative.Create(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) creativeVisibility(ctx *gin.Context, input setTaskVisibilityInput) (*httpx.NoData, error) {
	return nil, handler.services.Creative.Visibility(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Online)
}

func (handler *Handler) creativeWorks(ctx *gin.Context) ([]creative.WorkDTO, error) {
	return handler.services.Creative.Works(ctx.Request.Context(), "")
}

func (handler *Handler) creativeReview(ctx *gin.Context, input creative.ReviewInput) (*httpx.NoData, error) {
	return nil, handler.services.Creative.Review(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) creativeAward(ctx *gin.Context, input creative.LinksInput) (*httpx.NoData, error) {
	return nil, handler.services.Creative.Award(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) creativeReturn(ctx *gin.Context, input creative.Link) (*httpx.NoData, error) {
	return nil, handler.services.Creative.ReturnEvidence(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) creativeRoutes() []httpx.Route {
	return []httpx.Route{
		{
			Method:  http.MethodPost,
			Path:    "/creative/works/:id/return",
			Label:   "退回点赞截图",
			Handler: httpx.JSON(handler.creativeReturn),
		},
		{
			Method:  http.MethodGet,
			Path:    "/creative/tasks",
			Label:   "查看创作任务",
			Handler: httpx.Adapt(handler.creativeTasks),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/tasks",
			Label:   "新建创作任务",
			Handler: httpx.JSON(handler.creativeCreate),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/tasks/:id/visibility",
			Label:   "上下架创作任务",
			Handler: httpx.JSON(handler.creativeVisibility),
		},
		{
			Method:  http.MethodGet,
			Path:    "/creative/works",
			Label:   "查看创作作品",
			Handler: httpx.Adapt(handler.creativeWorks),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/works/:id/review",
			Label:   "审核创作作品",
			Handler: httpx.JSON(handler.creativeReview),
		},
		{
			Method:  http.MethodPost,
			Path:    "/creative/works/:id/award",
			Label:   "核定点赞奖励",
			Handler: httpx.JSON(handler.creativeAward),
		},
	}
}
