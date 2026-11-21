package handler

import (
	"baituber.cn/yanxuan/v2/internal/modules/wallet"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type withdrawInput struct {
	AmountFen string `json:"amount_fen"`
}

func (handler *Handler) wallet(ctx *gin.Context) (wallet.WalletDTO, error) {
	return handler.services.Wallet(ctx.Request.Context(), httpapi.CurrentUser(ctx))
}

func (handler *Handler) withdraw(ctx *gin.Context, input withdrawInput) (wallet.WithdrawalDTO, error) {
	return handler.services.Withdraw(
		ctx.Request.Context(), httpapi.CurrentUser(ctx), input.AmountFen, ctx.GetHeader("Idempotency-Key"),
	)
}
