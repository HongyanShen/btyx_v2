package handler

import (
	"baituber.cn/yanxuan/v2/internal/modules/notification"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

func (handler *Handler) recordChoices(ctx *gin.Context, input notification.ChoiceBatch) (*httpx.NoData, error) {
	return nil, handler.services.RecordChoices(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

type createFeedbackInput struct {
	Body string `json:"body"`
}

func (handler *Handler) createFeedback(ctx *gin.Context, input createFeedbackInput) (*httpx.NoData, error) {
	return nil, handler.services.CreateFeedback(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.Body)
}

func (handler *Handler) notifications(ctx *gin.Context) ([]notification.DTO, error) {
	return handler.services.Notifications(ctx.Request.Context(), httpapi.CurrentUser(ctx))
}

func (handler *Handler) readNotification(ctx *gin.Context) (*httpx.NoData, error) {
	return nil, handler.services.ReadNotification(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"))
}
