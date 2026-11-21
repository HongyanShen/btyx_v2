package handler

import (
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

func (handler *Handler) checkOwnedFile(ctx *gin.Context, id string) error {
	return handler.services.CheckOwnedFile(ctx.Request.Context(), httpapi.CurrentUser(ctx), id)
}
