package main

import (
	"fmt"
	"strconv"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"

	"baituber.cn/yanxuan/v2/internal/modules/wallet"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/gin-gonic/gin"
)

type pagination struct {
	Current    int
	Pages      []int
	Path       string
	Label      string
	TotalLabel string
	Legacy     bool
	Search     string
	Prev       int
	Next       int
}

func pageBounds(ctx *gin.Context, path string, length, size int) (int, int, pagination) {
	lastPage := max(1, (length+size-1)/size)
	page, _ := strconv.Atoi(ctx.Query("page"))
	page = min(max(page, 1), lastPage)
	start := (page - 1) * size
	pager := pagination{
		Path:    path,
		Current: page,
	}
	if lastPage > 1 {
		pager.Prev = page - 1
		if page < lastPage {
			pager.Next = page + 1
		}

		pager.Pages = pagerNumbers(page, lastPage)
	}

	return start, min(start+size, length), pager
}

func pagerNumbers(current, last int) []int {
	firstVisible := max(1, min(current-2, last-4))
	lastVisible := min(last, firstVisible+4)
	pages := make([]int, 0, 9)
	if firstVisible > 1 {
		pages = append(pages, 1)
		if firstVisible > 2 {
			pages = append(pages, 0)
		}
	}

	for page := firstVisible; page <= lastVisible; page++ {
		pages = append(pages, page)
	}

	if lastVisible < last {
		if lastVisible < last-1 {
			pages = append(pages, 0)
		}

		pages = append(pages, last)
	}

	return pages
}

func listPageBounds(ctx *gin.Context, path string, length, size int) (int, int, pagination) {
	start, end, pager := pageBounds(ctx, path, length, size)
	if length > 0 && len(pager.Pages) == 0 {
		pager.Pages = []int{1}
	}

	return start, end, pager
}

type withdrawalRow struct {
	wallet.WithdrawalDTO
	Amount string
	Label  string
	Date   string
	Paid   bool
}

type withdrawalPage struct {
	Rows    []withdrawalRow
	Pager   pagination
	Blocked bool
	URL     string
	Month   string
}

func (admin *admin) loadWithdrawals(ctx *gin.Context) (*withdrawalPage, error) {
	request := ctx.Request.Context()
	month := ctx.Query("month")
	var rows []wallet.WithdrawalDTO
	var err error
	if month == "" {
		rows, err = admin.services.Withdrawals(request, currentOperator(ctx).ID, enum.RoleAdmin)
	} else {
		rows, err = admin.services.WithdrawalsInMonth(request, month)
	}

	if err != nil {
		return nil, err
	}

	start, end, pager := pageBounds(ctx, "/withdrawals", len(rows), 12)
	pager.Legacy = true
	if month != "" {
		pager.Search = "&month=" + month
	}

	page := &withdrawalPage{
		Pager: pager,
		URL:   fmt.Sprintf("/withdrawals?page=%d%s", pager.Current, pager.Search),
		Month: month,
	}

	for _, row := range rows {
		if row.Status == enum.WithdrawalBlockedConfig {
			page.Blocked = true
			break
		}
	}

	for _, row := range rows[start:end] {
		amount, err := money.ParseFen(row.AmountFen)
		if err != nil {
			return nil, err
		}

		page.Rows = append(page.Rows, withdrawalRow{
			WithdrawalDTO: row,
			Amount:        amount.Shift(-2).StringFixed(2),
			Label:         label(withdrawalLabels[row.Status], string(row.Status)),
			Date:          dateText(row.CreatedAt),
			Paid:          row.Status == enum.WithdrawalSuccess,
		})
	}

	return page, nil
}

var beijing = time.FixedZone("CST", 8*60*60)

func dateText(date time.Time) string {
	return date.In(beijing).Format("2006/1/2 15:04:05")
}

func label(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

var withdrawalLabels = map[enum.WithdrawalStatus]string{
	enum.WithdrawalQueued:              "等待处理",
	enum.WithdrawalSubmitting:          "正在发起转账",
	enum.WithdrawalUnknown:             "转账结果待确认",
	enum.WithdrawalWaitingConfirmation: "待用户确认收款",
	enum.WithdrawalSuccess:             "已到账",
	enum.WithdrawalFailed:              "失败，已退回余额",
	enum.WithdrawalCancelled:           "已关闭，已退回余额",
	enum.WithdrawalBlockedConfig:       "等待开通转账通道",
}
