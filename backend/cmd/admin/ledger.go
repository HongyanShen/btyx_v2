package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var ledgerRoute = workspacePage{
	Path:     "/ledger",
	Label:    "余额流水",
	Endpoint: "GET /ledger",
}

const ledgerAdjustPermission = "POST /ledger/:id/adjust"

type ledgerPage struct {
	Rows      []ledgerViewRow
	Pager     pagination
	CanAdjust bool
	Dialog    *ledgerDialog
}

type ledgerViewRow struct {
	ID       string
	UserID   string
	Kind     string
	Amount   string
	Positive bool
	Reason   string
	Date     string
}

type ledgerDialog struct {
	ID     string
	Page   int
	CSRF   string
	Key    string
	Amount string
	Reason string
	Error  string
}

var ledgerKinds = map[enum.LedgerKind]string{
	enum.LedgerSettlementCredit:  "结算入账",
	enum.LedgerWithdrawalFreeze:  "提现冻结",
	enum.LedgerWithdrawalSuccess: "提现到账确认",
	enum.LedgerWithdrawalRefund:  "提现退回余额",
	enum.LedgerAdjustment:        "关联调整",
}

func (admin *admin) loadLedger(ctx *gin.Context) (*ledgerPage, error) {
	account := currentOperator(ctx)
	ledger, err := admin.services.Ledger(ctx.Request.Context(), account.ID, enum.RoleAdmin)
	if err != nil {
		return nil, err
	}

	start, end, pager := listPageBounds(ctx, ledgerRoute.Path, len(ledger), 12)
	page := &ledgerPage{
		Pager:     pager,
		CanAdjust: canCall(account, ledgerAdjustPermission),
	}
	for _, entry := range ledger[start:end] {
		amount, err := decimal.NewFromString(entry.AmountFen)
		if err != nil {
			return nil, err
		}

		amountText := "¥" + money.Yuan(amount)
		if !amount.IsNegative() {
			amountText = "+¥" + money.Yuan(amount)
		}

		page.Rows = append(page.Rows, ledgerViewRow{
			ID:       entry.ID,
			UserID:   entry.UserID,
			Kind:     label(ledgerKinds[entry.Kind], string(entry.Kind)),
			Amount:   amountText,
			Positive: !amount.IsNegative(),
			Reason:   label(entry.Reason, "—"),
			Date:     dateText(entry.CreatedAt),
		})
	}

	if id := ctx.Query("adjust"); id != "" && page.CanAdjust {
		for _, row := range page.Rows {
			if row.ID == id {
				page.Dialog = &ledgerDialog{
					ID:   id,
					Page: pager.Current,
					CSRF: csrfToken(sessionToken(ctx)),
					Key:  business.NewID(),
				}
				break
			}
		}
	}

	return page, nil
}

var ledgerYuan = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)

func (admin *admin) ledgerAdjust(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, ledgerRoute, ledgerAdjustPermission) {
		return
	}

	pageNumber, _ := strconv.Atoi(ctx.Query("page"))
	dialog := ledgerDialog{
		ID:     ctx.Param("id"),
		Page:   max(pageNumber, 1),
		CSRF:   csrfToken(sessionToken(ctx)),
		Key:    ctx.PostForm("key"),
		Amount: strings.TrimSpace(ctx.PostForm("delta_yuan")),
		Reason: ctx.PostForm("reason"),
	}
	amount, err := decimal.NewFromString(dialog.Amount)
	if err != nil || !ledgerYuan.MatchString(dialog.Amount) {
		err = app.Invalid("请输入最多两位小数的金额")
	} else {
		err = admin.services.AdjustLedger(ctx.Request.Context(), currentOperator(ctx).ID, dialog.ID, amount.Shift(2).String(), dialog.Reason, dialog.Key)
	}

	data := admin.workspace(ctx, ledgerRoute)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		dialog.Error = message
		if partialRequest(ctx) {
			ctx.Header("X-Admin-Form", "ledger")
			admin.render(ctx, status, "ledger-dialog", dialog)
			return
		}

		data.Ledger, err = admin.loadLedger(ctx)
		if err != nil {
			status, data.Error = adminErrorMessage(ctx, err)
		} else {
			data.Ledger.Dialog = &dialog
		}

		admin.renderWorkspace(ctx, status, data)
		return
	}

	if !partialRequest(ctx) {
		redirect(ctx, ledgerRoute.Path+"?page="+strconv.Itoa(dialog.Page))
		return
	}

	data.Ledger, err = admin.loadLedger(ctx)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		data.Error = message
		admin.renderWorkspace(ctx, status, data)
		return
	}

	data.Success = "已创建关联调整流水"
	admin.renderWorkspace(ctx, http.StatusOK, data)
}
