package operations

import (
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type adjustLedgerInput struct {
	DeltaFen string `json:"delta_fen"`
	Reason   string `json:"reason"`
}

func (handler *Handler) previewSettlement(ctx *gin.Context) (settlement.SettlementPreview, error) {
	return handler.services.PreviewSettlement(ctx.Request.Context(), ctx.Query("through"))
}

func (handler *Handler) settle(ctx *gin.Context, input settlement.SettlementInput) (settlement.BatchDTO, error) {
	input.Key = ctx.GetHeader("Idempotency-Key")

	return handler.services.Settle(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) settlements(ctx *gin.Context) ([]settlement.BatchDTO, error) {
	return handler.services.Batches(ctx.Request.Context())
}

func (handler *Handler) distributionSettings(ctx *gin.Context) (task.DistributionSettings, error) {
	return handler.services.DistributionSettings(ctx.Request.Context())
}

func (handler *Handler) updateDistributionSettings(ctx *gin.Context, input task.DistributionSettings) (task.DistributionSettings, error) {
	return handler.services.UpdateDistributionSettings(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) adjustLedger(ctx *gin.Context, input adjustLedgerInput) (*httpx.NoData, error) {
	return nil, handler.services.AdjustLedger(
		ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"),
		input.DeltaFen, input.Reason, ctx.GetHeader("Idempotency-Key"),
	)
}
