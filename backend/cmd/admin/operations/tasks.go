package operations

import (
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type setTaskVisibilityInput struct {
	Online bool `json:"online"`
}

type assignTaskInput struct {
	UserID string `json:"user_id"`
}

func (handler *Handler) tasks(ctx *gin.Context) ([]task.VersionDTO, error) {
	return handler.services.Tasks(ctx.Request.Context())
}

func (handler *Handler) marketingSettings(ctx *gin.Context) (task.MarketingSettingsDTO, error) {
	return handler.services.MarketingSettings(ctx.Request.Context())
}

func (handler *Handler) updateMarketingSettings(ctx *gin.Context, input task.MarketingSettingsInput) (task.MarketingSettingsDTO, error) {
	return handler.services.UpdateMarketingSettings(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) createTask(ctx *gin.Context, input task.TaskInput) (task.VersionDTO, error) {
	return handler.services.CreateTask(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) versions(ctx *gin.Context) ([]task.VersionDTO, error) {
	return handler.services.Versions(ctx.Request.Context(), ctx.Param("id"))
}

func (handler *Handler) publishVersion(ctx *gin.Context, input task.TaskInput) (task.VersionDTO, error) {
	return handler.services.PublishVersion(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) setTaskVisibility(ctx *gin.Context, input setTaskVisibilityInput) (*httpx.NoData, error) {
	return nil, handler.services.SetOnline(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Online)
}

func (handler *Handler) assignTask(ctx *gin.Context, input assignTaskInput) (task.AssignmentDTO, error) {
	return handler.services.Assign(
		ctx.Request.Context(), httpapi.CurrentUser(ctx), input.UserID,
		ctx.Param("id"), ctx.GetHeader("Idempotency-Key"),
	)
}
