package handler

import (
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type completeStepInput struct {
	Step enum.TaskStep `json:"step"`
}

func (handler *Handler) todayAvailability(ctx *gin.Context) (task.TodayAvailabilityDTO, error) {
	return handler.services.TodayAvailability(ctx.Request.Context())
}

func (handler *Handler) claimTask(ctx *gin.Context) (task.AssignmentDTO, error) {
	return handler.services.ClaimToday(
		ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.GetHeader("Idempotency-Key"),
	)
}

func (handler *Handler) completeStep(ctx *gin.Context, input completeStepInput) (*httpx.NoData, error) {
	return nil, handler.services.CompleteStep(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Step)
}

func (handler *Handler) submit(ctx *gin.Context, input submission.SubmitInput) (*httpx.NoData, error) {
	if err := handler.checkOwnedFile(ctx, input.Screenshot); err != nil {
		return nil, err
	}

	return nil, handler.services.Submit(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}
