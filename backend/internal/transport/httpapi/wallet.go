package httpapi

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/modules/wallet"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type walletHandler struct {
	service *wallet.Service
}

func WalletRoutes(service *wallet.Service) []httpx.Route {
	handler := &walletHandler{
		service: service,
	}
	return []httpx.Route{
		{
			Method:  http.MethodGet,
			Path:    "/withdrawals",
			Label:   "查看提现记录",
			Handler: httpx.Adapt(handler.listWithdrawals),
		},
		{
			Method:  http.MethodGet,
			Path:    "/ledger",
			Label:   "查看余额流水",
			Handler: httpx.Adapt(handler.listLedger),
		},
	}
}

func (handler *walletHandler) listWithdrawals(ctx *gin.Context) ([]wallet.WithdrawalDTO, error) {
	principal := CurrentPrincipal(ctx)
	return handler.service.Withdrawals(ctx.Request.Context(), principal.ID, principal.Role)
}

func (handler *walletHandler) listLedger(ctx *gin.Context) ([]wallet.LedgerDTO, error) {
	principal := CurrentPrincipal(ctx)
	return handler.service.Ledger(ctx.Request.Context(), principal.ID, principal.Role)
}
