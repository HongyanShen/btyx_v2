package handler

import (
	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type replyFeedbackInput struct {
	Reply string `json:"reply"`
}

type publishAgreementInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (handler *Handler) contents(ctx *gin.Context) ([]content.DTO, error) {
	return handler.services.Contents(ctx.Request.Context())
}

func (handler *Handler) createContent(ctx *gin.Context, input content.Input) (content.DTO, error) {
	return handler.services.CreateContent(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) updateContentTags(ctx *gin.Context, input content.TagsInput) (*httpx.NoData, error) {
	return nil, handler.services.UpdateContentTags(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) audits(ctx *gin.Context) ([]audit.DTO, error) {
	return handler.services.Audits(ctx.Request.Context())
}

func (handler *Handler) replyFeedback(ctx *gin.Context, input replyFeedbackInput) (*httpx.NoData, error) {
	return nil, handler.services.ReplyFeedback(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Reply)
}

func (handler *Handler) agreements(ctx *gin.Context) ([]agreement.DTO, error) {
	return handler.services.Agreements(ctx.Request.Context())
}

func (handler *Handler) publishAgreement(ctx *gin.Context, input publishAgreementInput) (*httpx.NoData, error) {
	return nil, handler.services.PublishAgreement(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.Title, input.Body)
}
