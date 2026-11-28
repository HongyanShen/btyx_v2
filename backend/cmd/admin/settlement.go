package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var settlementsRoute = workspacePage{
	Path:  "/settlements",
	Label: "分发结算",
}

type settlementPage struct {
	Category      string
	Month         string
	Latest        string
	MonthLabel    string
	OlderSelected bool
	Shortcuts     []contentTagChoice
	Months        []int
	KPI           []settlementKPI
	Weeks         []settlementWeek
	Detail        *settlementDetail
	Confirm       *settlementConfirmation
	Person        *distributorDialog
	Username      string
	CanReadLocal  bool
	CanSettings   bool
}

type settlementKPI struct {
	Label string
	Value string
	Note  string
	Link  string
}

type settlementWeek struct {
	settlement.WeekDTO
	Label        string
	Date         string
	Badge        string
	Tone         string
	Amount       string
	People       int
	Done         int
	Waiting      int
	WaitingLabel string
	Projects     string
	Due          bool
	Credit       string
	Export       bool
	NextCredit   string
	Groups       []settlementGroup
}

type settlementGroup struct {
	ID          string
	Owner       *identity.UserDTO
	Name        string
	Projects    []string
	Qualified   int
	Waiting     int
	Late        int
	Checks      []settlementCheck
	Prices      []string
	Amount      string
	Posted      string
	NeedsCredit bool
	Ready       bool
	lines       []settlement.WeekLineDTO
}

type settlementCheck struct {
	Text   string
	Danger bool
}

type settlementDetail struct {
	Title   string
	Account string
	Amount  string
	Waiting bool
	Rows    []settlementDetailRow
}

type settlementDetailRow struct {
	settlement.WeekLineDTO
	Date         string
	PlatformName string
	Amount       string
	Waiting      bool
	Check        settlementCheck
}

type settlementConfirmation struct {
	Month   string
	Through string
	Token   string
	Key     string
	CSRF    string
	Label   string
	Total   string
	Count   int
	Waiting bool
	Stale   bool
	Error   string
}

func (admin *admin) loadSettlement(ctx *gin.Context) (*settlementPage, error) {
	account := currentOperator(ctx)
	request := ctx.Request.Context()
	access := settlement.MonthAccess{
		Pending:     canCall(account, "GET /settlements/preview"),
		History:     canCall(account, "GET /settlements"),
		Withdrawals: canCall(account, "GET /withdrawals"),
		Screenshots: canCall(account, "GET /files/:id/download"),
	}
	if !access.Pending && !access.History {
		return nil, app.Forbidden("无权读取此页面的数据")
	}

	current := admin.services.Now().In(beijing)
	latest := current.Format("2006-01")
	month := label(ctx.Query("month"), latest)
	report, err := admin.services.Month(request, month, access)
	if err != nil {
		return nil, err
	}

	users := make(map[string]*identity.UserDTO)
	if canCall(account, "GET /users") {
		seen := make(map[string]bool)
		var ids []string
		for _, week := range report.Weeks {
			for _, line := range week.Lines {
				if !seen[line.UserID] {
					ids = append(ids, line.UserID)
					seen[line.UserID] = true
				}
			}
		}

		if len(ids) > 0 {
			profiles, err := admin.services.UsersByIDs(request, ids)
			if err != nil {
				return nil, err
			}

			for i := range profiles {
				users[profiles[i].ID] = &profiles[i]
			}
		}
	}

	previous := current.AddDate(0, 0, 1-current.Day()).AddDate(0, -1, 0).Format("2006-01")
	page := &settlementPage{
		Category:      ctx.Query("category"),
		Month:         month,
		Latest:        latest,
		MonthLabel:    settlementMonthLabel(month),
		OlderSelected: month != latest && month != previous,
		Username:      account.Username,
		CanReadLocal:  canCall(account, "GET /files/:id"),
		CanSettings:   canViewPage(account, distributionSettingsRoute) && canCall(account, readDistributionSettings),
	}
	for _, shortcut := range []string{latest, previous} {
		page.Shortcuts = append(page.Shortcuts, contentTagChoice{
			Key:      shortcut,
			Label:    settlementMonthLabel(shortcut),
			Selected: month == shortcut,
		})
	}

	for number := 1; number <= 12; number++ {
		page.Months = append(page.Months, number)
	}

	var paid, pending, waiting []settlement.WeekLineDTO
	canCredit := access.Pending && canCall(account, "POST /settlements")
	for _, week := range report.Weeks {
		category := page.Category
		if category == "creative" || category == "distribution" {
			var filtered []settlement.WeekLineDTO
			for _, line := range week.Lines {
				if line.Creative == (category == "creative") {
					filtered = append(filtered, line)
				}
			}

			week.Lines = filtered
		}

		view := newSettlementWeek(week, report.Today, users, canCredit && category == "", access.History)
		page.Weeks = append(page.Weeks, view)
		for _, line := range week.Lines {
			if line.CreditedAt != nil {
				paid = append(paid, line)
			}

			if line.Due && line.CreditedAt == nil && line.AmountFen != "0" {
				pending = append(pending, line)
			}

			if settlementWaiting(line) {
				waiting = append(waiting, line)
			}
		}
	}

	page.KPI = settlementKPIs(month, report, access, canViewPage(account, workspacePage{Path: "/withdrawals"}), paid, pending, waiting)
	mode := ctx.Query("dialog")
	if mode == "person" {
		id := ctx.Query("member")
		if users[id] == nil {
			return nil, app.NotFound()
		}

		profiles, err := admin.services.Distributors(request, id, canCall(account, enrollmentDocumentPermission))
		if err != nil {
			return nil, err
		}

		profile := distributorRow(profiles[0], nil)
		tasks, err := admin.services.TodayDistributorTasks(request, id)
		if err != nil {
			return nil, err
		}

		page.Person = distributorDialogView(ctx, profile, &distributorsPage{}, "details", tasks)
		return page, nil
	}

	for _, week := range page.Weeks {
		if week.Through != ctx.Query("through") {
			continue
		}

		if mode == "detail" {
			for _, group := range week.Groups {
				if group.ID == ctx.Query("member") {
					page.Detail = newSettlementDetail(week, group)
				}
			}
		} else if mode == "confirm" && week.Credit != "" {
			pending := settlementReady(week.Lines)
			page.Confirm = &settlementConfirmation{
				Month:   month,
				Through: week.Through,
				Token:   week.ReviewToken,
				Key:     business.NewID(),
				CSRF:    csrfToken(sessionToken(ctx)),
				Label:   week.Label,
				Total:   fmt.Sprintf("%d 人 · %s", settlementPeople(pending), settlementYuan(settlementAmount(pending))),
				Count:   len(pending),
				Waiting: week.Waiting > 0,
			}
		}
	}

	return page, nil
}

func settlementKPIs(month string, report settlement.MonthDTO, access settlement.MonthAccess, withdrawals bool, paid, pending, waiting []settlement.WeekLineDTO) []settlementKPI {
	monthNumber, _ := strconv.Atoi(month[5:])
	result := []settlementKPI{
		{
			Label: fmt.Sprintf("%d 月已入账", monthNumber),
			Value: "—",
			Note:  "无权查看",
		},
		{
			Label: "待核对",
			Value: "—",
			Note:  "无权查看",
		},
		{
			Label: "分发员已提现",
			Value: "—",
			Note:  "无权查看",
		},
		{
			Label: "等截图",
			Value: "—",
			Note:  "无权查看",
		},
	}
	if access.History {
		result[0].Value = settlementYuan(settlementAmount(paid))
		result[0].Note = fmt.Sprintf("%d 条", settlementTaskCount(paid))
	}

	if access.Pending {
		result[1].Value = settlementYuan(settlementAmount(pending))
		result[1].Note = fmt.Sprintf("%d 人", settlementPeople(pending))
		result[3].Value = fmt.Sprintf("%d 条", settlementTaskCount(waiting))
		result[3].Note = "晚交顺延到下一期"
	}

	if access.Withdrawals {
		amount, _ := decimal.NewFromString(report.WithdrawnFen)
		result[2].Value = settlementYuan(amount)
		result[2].Note = fmt.Sprintf("%d 笔到账", report.Withdrawals)
		if withdrawals {
			result[2].Note = "看提现记录 ›"
			result[2].Link = "/withdrawals?month=" + url.QueryEscape(month)
		}
	}

	return result
}

func settlementMonthLabel(month string) string {
	number, _ := strconv.Atoi(month[5:])
	return fmt.Sprintf("%s 年 %d 月", month[:4], number)
}

func settlementLink(month, through, member, mode string) string {
	params := url.Values{}
	params.Set("month", month)
	params.Set("through", through)
	params.Set("member", member)
	params.Set("dialog", mode)
	return settlementsRoute.Path + "?" + params.Encode()
}

func (admin *admin) renderSettlementDialog(ctx *gin.Context, status int, page *settlementPage) {
	switch {
	case page.Detail != nil:
		admin.render(ctx, status, "settlement-detail", page.Detail)
	case page.Confirm != nil:
		admin.render(ctx, status, "settlement-confirm", page.Confirm)
	case page.Person != nil:
		admin.render(ctx, status, "distributor-dialog", page.Person)
	default:
		admin.failure(ctx, http.StatusConflict, "结算记录已更新，请刷新后重试")
	}
}

func (admin *admin) confirmSettlement(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, settlementsRoute, "POST /settlements") || !admin.allowPageAction(ctx, settlementsRoute, "GET /settlements/preview") {
		return
	}

	input := settlement.SettlementInput{
		Through:     ctx.PostForm("through"),
		ReviewToken: ctx.PostForm("review_token"),
		Key:         ctx.PostForm("key"),
	}
	_, err := admin.services.Settle(ctx.Request.Context(), currentOperator(ctx).ID, input)
	if err == nil && !partialRequest(ctx) {
		redirect(ctx, settlementsRoute.Path+"?"+ctx.Request.URL.Query().Encode())
		return
	}

	status := http.StatusOK
	message := ""
	if err != nil {
		status, message = adminErrorMessage(ctx, err)
		query := ctx.Request.URL.Query()
		query.Set("dialog", "confirm")
		query.Set("through", input.Through)
		ctx.Request.URL.RawQuery = query.Encode()
	}

	data := admin.workspace(ctx, settlementsRoute)
	page, loadErr := admin.loadSettlement(ctx)
	if loadErr != nil {
		status, data.Error = adminErrorMessage(ctx, loadErr)
	} else if err != nil && page.Confirm != nil {
		page.Confirm.Stale = page.Confirm.Token != input.ReviewToken
		page.Confirm.Token = input.ReviewToken
		page.Confirm.Key = input.Key
		page.Confirm.Error = message
		if partialRequest(ctx) {
			ctx.Header("X-Admin-Form", "settlement")
			admin.renderSettlementDialog(ctx, status, page)
			return
		}

		data.Settlement = page
	} else if err != nil {
		data.Error = message
	} else {
		data.Settlement = page
		data.Success = "已确认入账，奖励已计入分发员可提现余额"
	}

	admin.renderWorkspace(ctx, status, data)
}

func settlementYuan(value decimal.Decimal) string {
	whole, cents, _ := strings.Cut(money.Yuan(value), ".")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}

	return "¥" + whole + "." + cents
}
