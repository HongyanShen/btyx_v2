package main

import (
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func (admin *admin) saveMarketingTask(ctx *gin.Context) {
	mode := "new"
	permission := "POST /tasks"
	action := ctx.Param("action")
	id := ctx.Param("id")
	switch action {
	case "versions":
		mode, permission = "edit", "POST /tasks/:id/versions"
	case "assign":
		mode, permission = "assign", "POST /tasks/:id/assign"
	case "":
	default:
		admin.failure(ctx, http.StatusNotFound, "页面不存在")
		return
	}

	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, tasksRoute, permission) {
		return
	}

	request := ctx.Request.Context()
	actor := currentOperator(ctx)
	var err error
	var success string
	switch mode {
	case "assign":
		if !admin.allowPageAction(ctx, tasksRoute, "GET /users") {
			return
		}
		key := ctx.PostForm("key")
		_, err = admin.services.Assign(request, actor.ID, ctx.PostForm("user_id"), id, key)
		success = "已指派，占用分发员当天的一条名额"
	case "new", "edit":
		input, inputErr := marketingFormInput(ctx)
		err = inputErr
		success = "已加入营销内容池"
		if err == nil && mode == "new" {
			_, err = admin.services.CreateTask(request, actor.ID, input)
		} else if err == nil {
			versions, versionErr := admin.services.Versions(request, id)
			err = versionErr
			if err == nil && (len(versions) == 0 || versions[0].ID != ctx.PostForm("version_id")) {
				err = app.Conflict("营销内容已更新，请刷新后重试")
			}

			if err == nil {
				original := versions[0]
				input.ContentID = original.ContentID
				input.Deadline = original.Deadline
				if splitMarketingBody(input.Body) == splitMarketingBody(original.Body) {
					input.Body = original.Body
				}

				_, err = admin.services.PublishVersion(request, actor.ID, id, input)
			}

			success = "已保存修改"
		}
	}

	if err != nil {
		admin.marketingActionError(ctx, mode, id, err)
		return
	}

	admin.finishMarketing(ctx, success)
}

func marketingFormInput(ctx *gin.Context) (task.TaskInput, error) {
	platforms := []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu}
	choice := ctx.PostForm("platform")
	if choice != "both" && !enum.Platform(choice).Valid() {
		return task.TaskInput{}, app.Invalid("请选择发布平台")
	}

	if choice != "both" {
		platforms = []enum.Platform{enum.Platform(choice)}
	}

	copy := ctx.PostForm("body")
	minimum := ctx.PostForm("minimum_topics")
	if strings.TrimSpace(copy) == "" {
		return task.TaskInput{}, app.Invalid("请填写分发语")
	}

	if minimum != "" {
		count, err := strconv.Atoi(minimum)
		if err != nil || count < 0 {
			return task.TaskInput{}, app.Invalid("话题数量请输入整数")
		}
	}

	body := []string{copy}
	if tags := strings.TrimSpace(ctx.PostForm("topics")); tags != "" {
		body = append(body, "TAG："+tags)
	}

	if mention := strings.TrimSpace(ctx.PostForm("mention")); mention != "" {
		body = append(body, "@账号："+mention)
	}

	if count, _ := strconv.Atoi(minimum); count > 0 {
		body = append(body, "话题至少带满 "+minimum+" 个（含必带 TAG）")
	}

	return task.TaskInput{
		Title:     ctx.PostForm("title"),
		Body:      strings.Join(body, "\n"),
		Project:   strings.TrimSpace(ctx.PostForm("project")),
		Files:     append([]string{}, ctx.PostFormArray("files")...),
		Platforms: platforms,
		Tags:      append([]string{}, ctx.PostFormArray("tags")...),
	}, nil
}

var marketingHours = regexp.MustCompile(`^[0-9]+(?:\.[05])?$`)

func (admin *admin) saveMarketingSettings(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, tasksRoute, marketingSettingsPermission) {
		return
	}

	mode := ctx.PostForm("mode")
	input, err := marketingSettingsInput(ctx, mode)
	if err == nil {
		_, err = admin.services.UpdateMarketingSettings(ctx.Request.Context(), currentOperator(ctx).ID, input)
	}

	if err != nil {
		admin.marketingActionError(ctx, mode, "", err)
		return
	}

	admin.finishMarketing(ctx, "营销设置已保存")
}

func marketingSettingsInput(ctx *gin.Context, mode string) (task.MarketingSettingsInput, error) {
	var input task.MarketingSettingsInput
	switch mode {
	case "price":
		value, err := marketingPriceInput(ctx.PostForm("price"))
		if err != nil {
			return input, err
		}
		price := value.String()
		input.PriceFen = &price
	case "visibility":
		online, err := strconv.ParseBool(ctx.PostForm("online"))
		if err != nil {
			return input, app.Invalid("请选择营销分发状态")
		}
		input.Online = &online
	case "expiry":
		expiry := enum.MarketingExpiryMode(ctx.PostForm("expiry_mode"))
		if !expiry.Valid() {
			return input, app.Invalid("请选择失效规则")
		}
		input.ExpiryMode = &expiry
		if expiry == enum.MarketingExpiryFixedDuration {
			raw := strings.TrimSpace(ctx.PostForm("hours"))
			hours, err := decimal.NewFromString(raw)
			if err != nil || !marketingHours.MatchString(raw) || hours.LessThan(decimal.RequireFromString("0.5")) || hours.GreaterThan(decimal.NewFromInt(24)) {
				return input, app.Invalid("固定时长须在 0.5 至 24 小时之间，以半小时为单位")
			}

			minutes := int(hours.Mul(decimal.NewFromInt(60)).IntPart())
			input.ExpiryMinutes = &minutes
		}
	default:
		return input, app.Invalid("请填写要修改的营销设置")
	}

	return input, nil
}

func (admin *admin) finishMarketing(ctx *gin.Context, success string) {
	if !partialRequest(ctx) {
		redirect(ctx, tasksRoute.Path+"?"+ctx.Request.URL.Query().Encode())
		return
	}

	data := admin.workspace(ctx, tasksRoute)
	page, err := admin.loadMarketing(ctx)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		data.Error = message
		admin.renderWorkspace(ctx, status, data)
		return
	}

	data.Tasks = page
	data.Success = success
	admin.renderWorkspace(ctx, http.StatusOK, data)
}

func (admin *admin) marketingActionError(ctx *gin.Context, mode, id string, actionErr error) {
	status, message := adminErrorMessage(ctx, actionErr)
	if mode == "expiry" {
		admin.failure(ctx, status, message)
		return
	}

	query := ctx.Request.URL.Query()
	query.Set("dialog", mode)
	query.Set("member", id)
	query.Set("price", ctx.PostForm("price"))
	ctx.Request.URL.RawQuery = query.Encode()
	page, err := admin.loadMarketing(ctx)
	if err != nil || page.Dialog == nil {
		admin.failure(ctx, status, message)
		return
	}

	form := page.Dialog
	form.Error = message
	if mode == "new" || mode == "edit" {
		form.Row.ID = ctx.PostForm("version_id")
		form.Row.Title = ctx.PostForm("title")
		form.Row.Project = ctx.PostForm("project")
		form.Row.Copy = marketingCopy{
			Text:    ctx.PostForm("body"),
			Tags:    ctx.PostForm("topics"),
			Mention: ctx.PostForm("mention"),
			Minimum: ctx.PostForm("minimum_topics"),
		}
		form.Row.Files = ctx.PostFormArray("files")
		form.Attachments = contentAttachments(form.Row.Files)
		form.MediaKind = ctx.PostForm("kind")
		form.Platform = ctx.PostForm("platform")
		selected := ctx.PostFormArray("tags")
		for i := range form.Choices {
			form.Choices[i].Selected = slices.Contains(selected, form.Choices[i].Key)
		}
	}

	if mode == "assign" {
		form.Key = ctx.PostForm("key")
		for i := range form.Editors {
			form.Editors[i].Selected = form.Editors[i].Key == ctx.PostForm("user_id")
		}
	}

	if partialRequest(ctx) {
		ctx.Header("X-Admin-Form", "marketing")
		admin.render(ctx, status, "marketing-dialog", form)
		return
	}

	data := admin.workspace(ctx, tasksRoute)
	data.Tasks = page
	admin.renderWorkspace(ctx, status, data)
}
