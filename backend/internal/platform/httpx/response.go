package httpx

import (
	"errors"
	"net/http"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
)

type Envelope[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func Abort(ctx *gin.Context, status int, message string) {
	ctx.AbortWithStatusJSON(status, Envelope[*NoData]{
		Code: status,
		Msg:  message,
		Data: nil,
	})
}

func Respond[T any](ctx *gin.Context, data T) {
	ctx.JSON(http.StatusOK, Envelope[T]{
		Code: 0,
		Msg:  "",
		Data: data,
	})
}

func RespondError(ctx *gin.Context, err error) {
	var businessError *app.Error
	if errors.As(err, &businessError) {
		logging.From(ctx.Request.Context()).Warn().
			Int("status", businessError.Status).
			Str("error_code", string(businessError.Code)).
			Msg("business request rejected")

		ctx.JSON(businessError.Status, Envelope[*NoData]{
			Code: businessError.Status,
			Msg:  businessError.Message,
			Data: nil,
		})

		return
	}

	_ = ctx.Error(err)
	logging.Error(ctx.Request.Context(), err).Msg("http request failed")

	ctx.JSON(http.StatusInternalServerError, Envelope[*NoData]{
		Code: http.StatusInternalServerError,
		Msg:  "服务暂时不可用，请稍后重试",
		Data: nil,
	})
}
