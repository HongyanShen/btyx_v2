package main

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var tasksRoute = workspacePage{
	PermissionLabel: "营销任务",
	Path:            "/tasks",
	Label:           "任务管理",
	Endpoint:        "GET /tasks",
}

const marketingSettingsPermission = "POST /marketing-settings"

type marketingPage struct {
	LegacySettings bool
	Filter         marketingFilter
	Artists        []string
	Dates          []contentTagChoice
	States         []marketingState
	Rows           []marketingRow
	Pager          pagination
	Settings       *marketingSettingsView
	CanCreate      bool
	CanEdit        bool
	CanPreview     bool
	CanReadLocal   bool
	Username       string
	CSRF           string
	Dialog         *marketingDialog
}

type marketingState struct {
	Text    string
	Count   int
	Current bool
}

type marketingFilter struct {
	Code   string
	Artist string
	Date   string
	Picked string
	State  string
	Page   int
}

type marketingSettingsView struct {
	task.MarketingSettingsDTO
	Price       string
	DoublePrice string
	Expiry      string
	EndOfDay    bool
	CanEdit     bool
	History     []string
}

type marketingRow struct {
	task.VersionDTO
	Artist        string
	UploadDate    string
	Media         string
	PlatformText  string
	State         string
	Next          bool
	Recipient     string
	RecipientNote string
	Expiry        string
	LastExpired   string
	Steps         []marketingStep
	StepName      string
	Copy          marketingCopy
	TagLabels     []string
	CanAssign     bool
}

type marketingStep struct {
	Label string
	Done  bool
	Class string
}

type marketingCopy struct {
	Text    string
	Tags    string
	Mention string
	Minimum string
	MinText string
}

type marketingDialog struct {
	Mode         string
	Row          marketingRow
	Settings     *marketingSettingsView
	Price        string
	NextOnline   bool
	Editors      []contentTagChoice
	Projects     []string
	Matches      []string
	Choices      []contentTagChoice
	Attachments  []contentAttachment
	MediaKind    string
	Platform     string
	CanUpload    bool
	CanPreview   bool
	CanCreateTag bool
	Key          string
	CSRF         string
	Action       string
	Error        string
}

func (admin *admin) loadMarketing(ctx *gin.Context) (*marketingPage, error) {
	request := ctx.Request.Context()
	account := currentOperator(ctx)
	filter := requestMarketingFilter(ctx)
	records, err := admin.services.TasksByCode(request, filter.Code)
	if err != nil {
		return nil, err
	}

	tags, err := admin.contentTagCatalog(ctx, account)
	if err != nil {
		return nil, err
	}

	names := make(map[string]string, len(tags))
	for _, tag := range tags {
		names[tag.Key] = tag.Name
	}

	users := make(map[string]identity.UserDTO)
	var eligible []identity.UserDTO
	if canCall(account, "GET /users") {
		profiles, err := admin.services.Users(request)
		if err != nil {
			return nil, err
		}

		for _, user := range profiles {
			users[user.ID] = user
			if user.Status == enum.EnrollmentApproved && user.Consented && !user.Blocked {
				eligible = append(eligible, user)
			}
		}
	}

	page := &marketingPage{
		LegacySettings: !canViewPage(account, distributionSettingsRoute) || !canCall(account, readDistributionSettings),
		Filter:         filter,
		CanCreate:      canCall(account, "POST /tasks"),
		CanEdit:        canCall(account, "POST /tasks/:id/versions"),
		CanPreview:     canCall(account, "GET /files/:id/download"),
		CanReadLocal:   canCall(account, "GET /files/:id"),
		Username:       account.Username,
		CSRF:           csrfToken(sessionToken(ctx)),
	}
	if canCall(account, "GET /marketing-settings") {
		settings, err := admin.services.MarketingSettings(request)
		if err != nil {
			return nil, err
		}

		page.Settings = marketingSettings(settings, canCall(account, marketingSettingsPermission))
	}

	today := admin.services.Now().In(beijing)
	canAssign := canCall(account, "POST /tasks/:id/assign") && canCall(account, "GET /users") && (page.Settings == nil || page.Settings.Online)
	all := make([]marketingRow, 0, len(records))
	var filtered []marketingRow
	counts := make(map[string]int)
	nextFound := false
	for _, version := range records {
		row := marketingRowView(version, users, names, today, canAssign)
		row.Next = !nextFound && version.NextInQueue && row.State == "排队中" && (page.Settings == nil || page.Settings.Online)
		nextFound = nextFound || row.Next
		all = append(all, row)
		if !slices.Contains(page.Artists, row.Artist) {
			page.Artists = append(page.Artists, row.Artist)
		}

		if !marketingDateMatches(row, page.Filter, today) || (page.Filter.Artist != "" && row.Artist != page.Filter.Artist) {
			continue
		}

		counts["全部"]++
		counts[row.State]++
		if page.Filter.State == "全部" || row.State == page.Filter.State {
			filtered = append(filtered, row)
		}
	}

	for _, text := range []string{"全部", "排队中", "已领取", "已发布"} {
		page.States = append(page.States, marketingState{
			Text:    text,
			Count:   counts[text],
			Current: page.Filter.State == text,
		})
	}

	page.Dates = marketingDates(page.Filter.Date)
	start, end, pager := pageBounds(ctx, tasksRoute.Path, len(filtered), 10)
	pager.Search = "&" + marketingQuery(page.Filter)
	pager.Label = "营销内容分页"
	page.Filter.Page = pager.Current
	page.Pager = pager
	page.Rows = filtered[start:end]
	if mode := ctx.Query("dialog"); mode != "" {
		page.Dialog, err = buildMarketingDialog(ctx, page, mode, all, eligible, tags)
	}

	return page, err
}

func requestMarketingFilter(ctx *gin.Context) marketingFilter {
	page, _ := strconv.Atoi(ctx.Query("page"))
	return marketingFilter{
		Code:   strings.TrimSpace(ctx.Query("task_code")),
		Artist: ctx.Query("artist"),
		Date:   label(ctx.Query("date_mode"), "all"),
		Picked: ctx.Query("picked"),
		State:  label(ctx.Query("state"), "全部"),
		Page:   max(page, 1),
	}
}

func marketingQuery(filter marketingFilter) string {
	query := url.Values{}
	if filter.Code != "" {
		query.Set("task_code", filter.Code)
	}

	query.Set("artist", filter.Artist)
	query.Set("date_mode", filter.Date)
	query.Set("picked", filter.Picked)
	query.Set("state", filter.State)
	return query.Encode()
}

func marketingLink(filter marketingFilter, id, mode string) string {
	return tasksRoute.Path + "?" + marketingQuery(filter) + "&page=" + strconv.Itoa(filter.Page) + "&member=" + url.QueryEscape(id) + "&dialog=" + url.QueryEscape(mode)
}

func marketingDates(current string) []contentTagChoice {
	keys := []string{"all", "today", "yesterday", "week", "pick"}
	labels := []string{"全部日期", "今天", "昨天", "近 7 天", "指定日期"}
	result := make([]contentTagChoice, len(keys))
	for i, key := range keys {
		result[i] = contentTagChoice{
			Key:      key,
			Label:    labels[i],
			Selected: key == current,
		}
	}

	return result
}

func marketingDateMatches(row marketingRow, filter marketingFilter, today time.Time) bool {
	switch filter.Date {
	case "today":
		return row.UploadDate == today.Format(time.DateOnly)
	case "yesterday":
		return row.UploadDate == today.AddDate(0, 0, -1).Format(time.DateOnly)
	case "week":
		return row.UploadDate >= today.AddDate(0, 0, -6).Format(time.DateOnly) && row.UploadDate <= today.Format(time.DateOnly)
	case "pick":
		return filter.Picked == "" || row.UploadDate == filter.Picked
	default:
		return true
	}
}

func marketingSettings(settings task.MarketingSettingsDTO, canEdit bool) *marketingSettingsView {
	price, _ := decimal.NewFromString(settings.PriceFen)
	view := &marketingSettingsView{
		MarketingSettingsDTO: settings,
		Price:                money.Yuan(price),
		DoublePrice:          money.Yuan(price.Add(price)),
		Expiry:               strconv.Itoa(settings.ExpiryMinutes / 60),
		CanEdit:              canEdit,
		EndOfDay:             settings.ExpiryMode == enum.MarketingExpiryEndOfDay,
	}
	if settings.ExpiryMinutes%60 != 0 {
		view.Expiry += ".5"
	}

	for _, item := range settings.PriceChanges {
		before, _ := decimal.NewFromString(item.BeforeFen)
		after, _ := decimal.NewFromString(item.AfterFen)
		view.History = append(view.History, item.ChangedAt.In(beijing).Format("01/02 15:04")+" · "+item.Operator+" · ¥"+money.Yuan(before)+" → ¥"+money.Yuan(after))
	}

	return view
}

func buildMarketingDialog(ctx *gin.Context, page *marketingPage, mode string, rows []marketingRow, users []identity.UserDTO, tags []enrollment.MarketingTagDTO) (*marketingDialog, error) {
	account := currentOperator(ctx)
	form := &marketingDialog{
		Mode:         mode,
		Settings:     page.Settings,
		CanUpload:    canCall(account, "POST /files/uploads") && canCall(account, "POST /files/:id/complete"),
		CanPreview:   page.CanPreview,
		CanCreateTag: canCall(account, "POST /marketing-tags") && canCall(account, "GET /marketing-tags"),
		CSRF:         page.CSRF,
		MediaKind:    "video",
		Platform:     "both",
		Key:          business.NewID(),
	}
	for _, project := range page.Artists {
		if project != "未分项目" {
			form.Projects = append(form.Projects, project)
		}
	}

	query := "?" + marketingQuery(page.Filter) + "&page=" + strconv.Itoa(page.Filter.Page)
	if mode == "price" || mode == "visibility" {
		if page.Settings == nil || !page.Settings.CanEdit {
			return nil, app.Forbidden("当前账号无权修改营销分发设置")
		}

		form.Action = "/tasks/settings" + query
		form.NextOnline = !page.Settings.Online
		if mode == "price" {
			value, err := marketingPriceInput(ctx.Query("price"))
			if err != nil {
				return nil, err
			}

			form.Price = money.Yuan(value)
			if form.Price == page.Settings.Price {
				return nil, nil
			}
		}

		return form, nil
	}

	if mode == "new" && page.CanCreate {
		form.Action = "/tasks" + query
		form.Row.Copy.Minimum = "5"
		if len(form.Projects) > 0 {
			form.Row.Project = form.Projects[0]
		}

		form.Choices = enrollmentTagChoices(tags, nil, nil)
		return form, nil
	}

	id := ctx.Query("member")
	index := slices.IndexFunc(rows, func(row marketingRow) bool { return row.TaskID == id })
	if index < 0 {
		return nil, app.NotFound()
	}

	form.Row = rows[index]
	switch mode {
	case "detail":
		form.Matches = marketingMatches(form.Row.Tags, users, tags)
	case "edit":
		if !page.CanEdit {
			return nil, app.Forbidden("当前账号无权修改营销内容")
		}
		form.Action = "/tasks/" + id + "/versions" + query
		form.Choices = enrollmentTagChoices(tags, form.Row.Tags, form.Row.Tags)
		form.Attachments = contentAttachments(form.Row.Files)
		if len(form.Row.Files) > 0 && !slices.Contains(form.Row.MediaTypes, enum.FileMP4) {
			form.MediaKind = "images"
		}
		if len(form.Row.Platforms) == 1 {
			form.Platform = string(form.Row.Platforms[0])
		}
	case "assign":
		if !form.Row.CanAssign {
			return nil, app.Forbidden("当前任务不可指派")
		}
		form.Action = "/tasks/" + id + "/assign" + query
		for _, user := range users {
			name := label(user.Name, user.Nickname)
			if user.Douyin != "" {
				name = label(name+" · "+user.Douyin, user.Douyin)
			}

			form.Editors = append(form.Editors, contentTagChoice{
				Key:   user.ID,
				Label: label(name, user.ID),
			})
		}
	default:
		return nil, app.NotFound()
	}

	return form, nil
}

var marketingAmount = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)

func marketingPriceInput(raw string) (decimal.Decimal, error) {
	value := strings.TrimSpace(raw)
	amount, err := decimal.NewFromString(value)
	if err != nil || !marketingAmount.MatchString(value) || !amount.IsPositive() || amount.GreaterThan(decimal.NewFromInt(500)) {
		return decimal.Zero, app.Invalid("每平台单价须大于 0 元且不超过 500 元")
	}

	return amount.Shift(2), nil
}
