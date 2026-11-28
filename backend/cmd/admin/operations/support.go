package operations

import (
	"strconv"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type replyFeedbackInput struct {
	Reply string `json:"reply"`
}

type publishAgreementInput struct {
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	BodyHTML *string `json:"body_html"`
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
	request := ctx.Request.Context()
	filter := audit.Filter{
		ActorName: ctx.Query("actor_name"),
		Endpoint:  ctx.Query("endpoint"),
		Platform:  enum.AuditPlatform(ctx.Query("platform")),
	}
	if !ctx.Request.URL.Query().Has("page") && filter.Endpoint == "" && filter.Platform == "" && filter.ActorName == "" && ctx.Query("start") == "" && ctx.Query("before") == "" {
		return handler.services.Audits(request)
	}

	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return nil, app.Invalid("分页参数无效")
	}

	if start := ctx.Query("start"); start != "" {
		filter.Start, err = time.Parse(time.RFC3339, start)
		if err != nil {
			return nil, app.Invalid("开始时间须带时区")
		}
	}

	if before := ctx.Query("before"); before != "" {
		filter.Before, err = time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, app.Invalid("结束时间须带时区")
		}
	}

	result, err := handler.services.AuditPage(request, filter, page)
	if err != nil {
		return nil, err
	}

	ctx.Header("X-Total-Count", strconv.Itoa(result.Total))
	return result.Rows, nil
}

func (handler *Handler) replyFeedback(ctx *gin.Context, input replyFeedbackInput) (*httpx.NoData, error) {
	return nil, handler.services.ReplyFeedback(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Reply)
}

func (handler *Handler) agreements(ctx *gin.Context) ([]agreement.DTO, error) {
	return handler.services.Agreements(ctx.Request.Context())
}

func (handler *Handler) publishAgreement(ctx *gin.Context, input publishAgreementInput) (*httpx.NoData, error) {
	if input.BodyHTML != nil {
		return nil, handler.services.PublishRichAgreement(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.Title, *input.BodyHTML)
	}

	return nil, handler.services.PublishAgreement(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.Title, input.Body)
}
