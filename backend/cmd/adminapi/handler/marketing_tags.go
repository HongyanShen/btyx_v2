package handler

import (
	"time"

	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type marketingTagInput struct {
	Name         string `json:"name"`
	PreviousName string `json:"previous_name"`
}

type deleteMarketingTagInput struct {
	UpdatedAt time.Time `json:"updated_at"`
}

func (handler *Handler) marketingTags(ctx *gin.Context) ([]enrollment.MarketingTagDTO, error) {
	return handler.services.MarketingTags(ctx.Request.Context())
}

func (handler *Handler) createMarketingTag(ctx *gin.Context, input marketingTagInput) (enrollment.MarketingTagDTO, error) {
	return handler.services.CreateMarketingTag(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.Name)
}

func (handler *Handler) renameMarketingTag(ctx *gin.Context, input marketingTagInput) (*httpx.NoData, error) {
	return nil, handler.services.RenameMarketingTag(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Name, input.PreviousName)
}

func (handler *Handler) deleteMarketingTag(ctx *gin.Context, input deleteMarketingTagInput) (*httpx.NoData, error) {
	return nil, handler.services.DeleteMarketingTag(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.UpdatedAt)
}
