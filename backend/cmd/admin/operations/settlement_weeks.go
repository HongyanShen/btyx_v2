package operations

import (
	"slices"
	"time"

	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

func (handler *Handler) settlementWeeks(ctx *gin.Context) (settlement.MonthDTO, error) {
	account := httpapi.CurrentPrincipal(ctx).Operator
	access := settlement.MonthAccess{
		Pending:     account.SuperAdmin || slices.Contains(account.Endpoints, "GET /settlements/preview"),
		History:     account.SuperAdmin || slices.Contains(account.Endpoints, "GET /settlements"),
		Withdrawals: account.SuperAdmin || slices.Contains(account.Endpoints, "GET /withdrawals"),
		Screenshots: account.SuperAdmin || slices.Contains(account.Endpoints, "GET /files/:id/download"),
	}
	if !access.Pending && !access.History {
		return settlement.MonthDTO{}, app.Forbidden("当前账号无权查看分发结算")
	}

	ctx.Header("Cache-Control", "private, no-store")
	return handler.readSettlementMonth(ctx, ctx.Query("month"), access)
}

func (handler *Handler) readSettlementMonth(ctx *gin.Context, month string, access settlement.MonthAccess) (settlement.MonthDTO, error) {
	request := ctx.Request.Context()
	report, err := handler.services.Month(request, month, access)
	if err != nil {
		return settlement.MonthDTO{}, err
	}

	account := httpapi.CurrentPrincipal(ctx).Operator
	if account.SuperAdmin || slices.Contains(account.Endpoints, "GET /users") {
		unique := make(map[string]bool)
		var ids []string
		for _, week := range report.Weeks {
			for _, line := range week.Lines {
				if !unique[line.UserID] {
					ids = append(ids, line.UserID)
					unique[line.UserID] = true
				}
			}
		}

		if len(ids) > 0 {
			report.Distributors, err = handler.services.UsersByIDs(request, ids)
		}
	}

	return report, err
}

func (handler *Handler) exportSettlement(ctx *gin.Context) (httpx.DownloadResponse, error) {
	account := httpapi.CurrentPrincipal(ctx).Operator
	if !account.SuperAdmin && !slices.Contains(account.Endpoints, "GET /settlements") {
		return httpx.DownloadResponse{}, app.Forbidden("当前账号无权导出结算明细")
	}

	through := ctx.Query("through")
	date, err := time.Parse(time.DateOnly, through)
	if err != nil {
		return httpx.DownloadResponse{}, app.Invalid("结算日期无效")
	}

	report, err := handler.readSettlementMonth(ctx, date.Format("2006-01"), settlement.MonthAccess{History: true})
	if err != nil {
		return httpx.DownloadResponse{}, err
	}

	for _, week := range report.Weeks {
		if week.Through != through || len(week.Lines) == 0 {
			continue
		}

		data, err := settlement.Workbook(week, report.Distributors)
		if err != nil {
			return httpx.DownloadResponse{}, err
		}

		return httpx.DownloadResponse{
			Data:      data,
			Name:      "营销入账明细_" + week.Start + "–" + week.End + ".xlsx",
			MediaType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		}, nil
	}

	return httpx.DownloadResponse{}, app.Invalid("该期暂无已入账明细")
}
