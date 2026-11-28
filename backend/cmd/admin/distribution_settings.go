package main

import (
	"net/http"
	"strconv"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"github.com/gin-gonic/gin"
)

const distributionSettingsPath = "/distribution/settings"
const readDistributionSettings = "GET " + distributionSettingsPath
const writeDistributionSettings = "POST " + distributionSettingsPath

var distributionSettingsRoute = workspacePage{
	Path:     distributionSettingsPath,
	Label:    "营销设置",
	Endpoint: readDistributionSettings,
}

type distributionSettingsPage struct {
	task.DistributionSettings
	CanManageCreative bool
	Marketing         *marketingSettingsView
	History           []task.SettingsChange
	HistoryPage       int
	HasMore           bool
	CanEdit           bool
	CSRF              string
	Hours             string
	Error             string
	Weekdays          []distributionWeekday
}

type distributionWeekday struct {
	Number int
	Name   string
}

func (admin *admin) loadDistributionSettings(ctx *gin.Context) (*distributionSettingsPage, error) {
	settings, err := admin.services.DistributionSettings(ctx.Request.Context())
	if err != nil {
		return nil, err
	}

	page := distributionSettingsView(ctx, settings)
	err = admin.addMarketingControls(ctx, page)
	return page, err
}

func distributionSettingsView(ctx *gin.Context, settings task.DistributionSettings) *distributionSettingsPage {
	page := &distributionSettingsPage{
		DistributionSettings: settings,
		CanEdit:              canCall(currentOperator(ctx), writeDistributionSettings),
		CSRF:                 csrfToken(sessionToken(ctx)),
		Hours:                strconv.FormatFloat(float64(settings.ExpiryMinutes)/60, 'f', -1, 64),
	}
	for index, name := range []string{"星期一", "星期二", "星期三", "星期四", "星期五", "星期六", "星期日"} {
		page.Weekdays = append(page.Weekdays, distributionWeekday{
			Number: index + 1,
			Name:   name,
		})
	}

	return page
}

func (admin *admin) saveDistributionSettings(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, distributionSettingsRoute, readDistributionSettings) ||
		!admin.allowPageAction(ctx, distributionSettingsRoute, writeDistributionSettings) {
		return
	}

	request := ctx.Request.Context()
	settings, err := admin.services.DistributionSettings(request)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		admin.failure(ctx, status, message)
		return
	}

	draft, err := distributionSettingsInput(ctx, settings)
	if err == nil {
		settings, err = admin.services.UpdateDistributionSettings(request, currentOperator(ctx).ID, draft)
	}

	if err == nil && !partialRequest(ctx) {
		redirect(ctx, distributionSettingsPath)
		return
	}

	data := admin.workspace(ctx, distributionSettingsRoute)
	status := http.StatusOK
	if err == nil {
		data.Distribution = distributionSettingsView(ctx, settings)
		data.Success = "分发设置已保存"
	} else {
		data.Distribution = distributionSettingsView(ctx, draft)
		status, data.Distribution.Error = adminErrorMessage(ctx, err)
		ctx.Header("X-Admin-Form", "distribution-settings")
	}

	if err := admin.addMarketingControls(ctx, data.Distribution); err != nil {
		code, msg := adminErrorMessage(ctx, err)
		admin.failure(ctx, code, msg)
		return
	}

	admin.renderWorkspace(ctx, status, data)
}

func distributionSettingsInput(ctx *gin.Context, settings task.DistributionSettings) (task.DistributionSettings, error) {
	schedule := &settings.Schedule
	schedule.SettlementFrequency = enum.SettlementFrequency(ctx.PostForm("settlement_frequency"))
	schedule.SettlementWeekday, _ = strconv.Atoi(ctx.PostForm("settlement_weekday"))
	schedule.SettlementMonthDay, _ = strconv.Atoi(ctx.PostForm("settlement_month_day"))
	schedule.SettlementClock = ctx.PostForm("settlement_clock")
	schedule.SettlementDelayMinutes, _ = strconv.Atoi(ctx.PostForm("settlement_delay_minutes"))
	schedule.PlaybackCutoffClock = ctx.PostForm("playback_cutoff_clock")
	schedule.PlaybackCutoffMinutes, _ = strconv.Atoi(ctx.PostForm("playback_cutoff_minutes"))
	schedule.PlaybackDeferralPeriods, _ = strconv.Atoi(ctx.PostForm("playback_deferral_periods"))
	settings.ExpiryMode = enum.MarketingExpiryMode(ctx.PostForm("expiry_mode"))
	input, err := marketingSettingsInput(ctx, "expiry")
	if err != nil {
		return settings, err
	}

	if input.ExpiryMinutes != nil {
		settings.ExpiryMinutes = *input.ExpiryMinutes
	}

	return settings, nil
}

func (admin *admin) addMarketingControls(ctx *gin.Context, page *distributionSettingsPage) error {
	account := currentOperator(ctx)
	if !canCall(account, "GET /marketing-settings") {
		return nil
	}

	settings, err := admin.services.MarketingSettings(ctx.Request.Context())
	if err != nil {
		return err
	}

	page.CanManageCreative = canViewPage(account, creativeTasksRoute) && canCall(account, creativeTasksRoute.Endpoint)
	page.Marketing = marketingSettings(settings, canCall(account, marketingSettingsPermission))
	page.HistoryPage, _ = strconv.Atoi(ctx.Query("history_page"))
	page.HistoryPage = max(1, min(page.HistoryPage, 1000000))
	page.History, page.HasMore, err = admin.services.MarketingHistory(ctx.Request.Context(), page.HistoryPage)
	if err != nil {
		return err
	}

	return nil
}

func (admin *admin) saveDispatchControls(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, distributionSettingsRoute, marketingSettingsPermission) ||
		!admin.allowPageAction(ctx, distributionSettingsRoute, "GET /marketing-settings") {
		return
	}

	price, err := marketingPriceInput(ctx.PostForm("price"))
	if err != nil {
		admin.failure(ctx, http.StatusBadRequest, "请填写有效的每平台单价")
		return
	}

	daily, err := strconv.Atoi(ctx.PostForm("daily_limit"))
	if err != nil || daily < 1 {
		admin.failure(ctx, http.StatusBadRequest, "分发每日名额须为正整数")
		return
	}

	creative, err := strconv.Atoi(ctx.PostForm("creative_limit"))
	if err != nil || creative < 1 {
		admin.failure(ctx, http.StatusBadRequest, "创作每日名额须为正整数")
		return
	}

	online := ctx.PostForm("online") == "on"
	creativeOnline := ctx.PostForm("creative_online") == "on"
	fen := price.String()
	input := task.MarketingSettingsInput{
		DailyLimit:     &daily,
		CreativeLimit:  &creative,
		CreativeOnline: &creativeOnline,
		Online:         &online,
	}
	if ctx.PostForm("original_price") != ctx.PostForm("price") {
		input.PriceFen = &fen
	}

	_, err = admin.services.UpdateMarketingSettings(ctx.Request.Context(), currentOperator(ctx).ID, input)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		admin.failure(ctx, status, message)
		return
	}

	redirect(ctx, distributionSettingsPath)
}
