package main

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var reviewsRoute = workspacePage{
	PermissionLabel: "执行与审核",
	Path:            "/reviews",
	Label:           "发布链接审核",
	Endpoint:        "GET /assignments",
}

var platformNames = map[enum.Platform]string{
	enum.PlatformDouyin:      "抖音",
	enum.PlatformXiaohongshu: "小红书",
}

var submissionStateLabels = map[enum.SubmissionStatus]string{
	enum.SubmissionPending:  "待审核",
	enum.SubmissionApproved: "已通过",
	enum.SubmissionRejected: "已驳回",
	enum.SubmissionInvalid:  "不予结算",
	enum.SubmissionWaived:   "素材问题，照常结算",
}

type reviewsPage struct {
	Rows         []reviewRow
	Pager        pagination
	Statuses     []contentTagChoice
	Platforms    []contentTagChoice
	Drawer       *reviewDrawer
	Dialog       *reviewDialog
	Person       *distributorDialog
	Username     string
	CanReadLocal bool
}

type reviewRow struct {
	task.AssignmentDTO
	Owner         *identity.UserDTO
	OwnerName     string
	SourceText    string
	StatusText    string
	Latest        string
	LatestISO     string
	Uniform       bool
	CanReview     bool
	CanFlag       bool
	PlatformsView []reviewPlatform
	Results       []reviewPlatform
}

type reviewPlatform struct {
	task.SubmissionDTO
	PlatformName string
	StatusText   string
	Color        string
	RevisionKey  string
	Link         string
	Selected     bool
}

type reviewDrawer struct {
	Row           reviewRow
	Pager         pagination
	Reward        string
	CanScreenshot bool
	CanRevoke     bool
}

type reviewDialog struct {
	Row      reviewRow
	Action   string
	Title    string
	Confirm  string
	FormURL  string
	CSRF     string
	Choices  []reviewPlatform
	Names    string
	Reason   string
	Selected []string
	KeepPay  bool
	Error    string
}

func (admin *admin) loadReviews(ctx *gin.Context) (*reviewsPage, error) {
	request := ctx.Request.Context()
	account := currentOperator(ctx)
	number, _ := strconv.Atoi(ctx.Query("page"))
	number = max(number, 1)
	status := ctx.Query("status")
	platform := ctx.Query("platform")
	records, total, err := admin.services.ReviewPage(request, number, enum.AssignmentStatus(status), enum.Platform(platform))
	if err != nil {
		return nil, err
	}

	if number > 1 && len(records) == 0 {
		number = max(1, (total+11)/12)
		records, total, err = admin.services.ReviewPage(request, number, enum.AssignmentStatus(status), enum.Platform(platform))
		if err != nil {
			return nil, err
		}
	}

	users := make(map[string]*identity.UserDTO)
	if canCall(account, "GET /users") {
		profiles, err := admin.services.Distributors(request, "", false)
		if err != nil {
			return nil, err
		}

		for i := range profiles {
			users[profiles[i].ID] = &profiles[i]
		}
	}

	page := &reviewsPage{
		Pager:        reviewPager(number, total, status, platform),
		Username:     account.Username,
		CanReadLocal: canCall(account, "GET /files/:id"),
	}
	for _, value := range []enum.AssignmentStatus{enum.AssignmentInProgress, enum.AssignmentPendingReview, enum.AssignmentRejected, enum.AssignmentPendingSettlement, enum.AssignmentSettled, enum.AssignmentRevoked, enum.AssignmentExpired, enum.AssignmentNotPayable} {
		page.Statuses = append(page.Statuses, contentTagChoice{
			Key:      string(value),
			Label:    assignmentStateLabels[value],
			Selected: status == string(value),
		})
	}

	for _, value := range []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu} {
		page.Platforms = append(page.Platforms, contentTagChoice{
			Key:      string(value),
			Label:    platformNames[value],
			Selected: platform == string(value),
		})
	}

	for _, assignment := range records {
		page.Rows = append(page.Rows, makeReviewRow(assignment, users[assignment.UserID], account))
	}

	if err := admin.loadReviewOverlay(ctx, page, account); err != nil {
		return nil, err
	}

	return page, nil
}

func reviewPager(number, total int, status, platform string) pagination {
	last := max(1, (total+11)/12)
	pager := pagination{
		Current:    number,
		Path:       reviewsRoute.Path,
		Label:      "发布链接审核分页",
		TotalLabel: "共 " + strconv.Itoa(total) + " 条",
		Search:     "&status=" + url.QueryEscape(status) + "&platform=" + url.QueryEscape(platform),
		Prev:       number - 1,
	}
	if number < last {
		pager.Next = number + 1
	}

	pager.Pages = pagerNumbers(number, last)

	return pager
}

func makeReviewRow(assignment task.AssignmentDTO, user *identity.UserDTO, account identity.OperatorDTO) reviewRow {
	row := reviewRow{
		AssignmentDTO: assignment,
		Owner:         user,
		OwnerName:     "分发员",
		StatusText:    assignmentStateLabels[assignment.Status],
		SourceText:    assignmentSourceLabels[assignment.Source],
		Latest:        "—",
		Uniform:       true,
	}
	if user != nil {
		row.OwnerName = label(user.Name, label(user.Nickname, "分发员"))
	}

	var latest time.Time
	for _, item := range assignment.Submissions {
		result := reviewSubmissionView(item)
		row.Results = append(row.Results, result)
		row.Uniform = row.Uniform && item.Status == assignment.Submissions[0].Status && item.Settled == assignment.Submissions[0].Settled && item.Reason == assignment.Submissions[0].Reason
		row.CanReview = row.CanReview || item.Status == enum.SubmissionPending
		row.CanFlag = row.CanFlag || item.Status == enum.SubmissionApproved && !item.Settled
		if item.SubmittedAt != nil && item.SubmittedAt.After(latest) {
			latest = *item.SubmittedAt
		}
	}

	row.CanReview = row.CanReview && canCall(account, "POST /assignments/:id/review")
	row.CanFlag = row.CanFlag && canCall(account, "POST /assignments/:id/reissue")
	if !latest.IsZero() {
		row.Latest = latest.In(beijing).Format("01-02 15:04")
		row.LatestISO = latest.UTC().Format(time.RFC3339Nano)
	}

	for _, platform := range assignment.Platforms {
		entry := reviewPlatform{PlatformName: platformNames[platform]}
		index := slices.IndexFunc(row.Results, func(item reviewPlatform) bool { return item.Platform == platform })
		if index >= 0 {
			entry = row.Results[index]
		}

		row.PlatformsView = append(row.PlatformsView, entry)
	}

	return row
}

func reviewSubmissionView(item task.SubmissionDTO) reviewPlatform {
	view := reviewPlatform{
		SubmissionDTO: item,
		PlatformName:  platformNames[item.Platform],
		StatusText:    submissionStateLabels[item.Status],
		Color:         "rejected",
		RevisionKey:   string(item.Platform) + ":" + strconv.FormatInt(item.Revision, 10),
	}
	if item.Settled {
		view.StatusText = "已结算"
	}

	if item.Settled || item.Status == enum.SubmissionPending {
		view.Color = "waiting"
	} else if item.Status == enum.SubmissionApproved || item.Status == enum.SubmissionWaived {
		view.Color = "passed"
	}

	if strings.HasPrefix(strings.ToLower(item.URL), "https://") || strings.HasPrefix(strings.ToLower(item.URL), "http://") {
		view.Link = item.URL
	}

	return view
}

func reviewLink(pager pagination, id, action, platform, drawer string) string {
	return "/reviews?page=" + strconv.Itoa(pager.Current) + pager.Search +
		"&assignment=" + url.QueryEscape(id) + "&action=" + url.QueryEscape(action) + "&selected_platform=" + url.QueryEscape(platform) + "&drawer=" + url.QueryEscape(drawer)
}

func (admin *admin) loadReviewOverlay(ctx *gin.Context, page *reviewsPage, account identity.OperatorDTO) error {
	drawerID := ctx.Query("drawer")
	id := ctx.Query("assignment")
	action := ctx.Query("action")
	if action == "person" {
		if id == "" {
			return app.NotFound()
		}

		if !canCall(account, "GET /users") {
			return app.Forbidden("当前账号无权查看分发员资料")
		}

		profiles, err := admin.services.Distributors(ctx.Request.Context(), id, canCall(account, enrollmentDocumentPermission))
		if err != nil {
			return err
		}

		tasks, err := admin.services.TodayDistributorTasks(ctx.Request.Context(), id)
		if err != nil {
			return err
		}

		page.Person = distributorDialogView(ctx, distributorRow(profiles[0], nil), &distributorsPage{}, "details", tasks)
		return nil
	}

	for _, row := range page.Rows {
		if row.ID == drawerID {
			amount, err := decimal.NewFromString(row.PriceFen)
			if err != nil {
				return err
			}

			page.Drawer = &reviewDrawer{
				Row:           row,
				Pager:         page.Pager,
				Reward:        "¥" + money.Yuan(amount),
				CanScreenshot: canCall(account, "GET /files/:id/download"),
				CanRevoke:     canCall(account, "POST /assignments/:id/revoke") && row.Source == enum.SourceAssigned && len(row.Results) == 0 && row.Status != enum.AssignmentRevoked,
			}
		}

		if row.ID == id && action != "" {
			page.Dialog = buildReviewDialog(ctx, account, page.Pager, row, action)
		}
	}

	return nil
}

func buildReviewDialog(ctx *gin.Context, account identity.OperatorDTO, pager pagination, row reviewRow, action string) *reviewDialog {
	dialog := &reviewDialog{
		Row:     row,
		Action:  action,
		CSRF:    csrfToken(sessionToken(ctx)),
		FormURL: "/reviews/" + row.ID + "/" + action + "?page=" + strconv.Itoa(pager.Current) + pager.Search + "&drawer=" + url.QueryEscape(ctx.Query("drawer")) + "&selected_platform=" + url.QueryEscape(ctx.Query("selected_platform")),
	}
	switch action {
	case "approve":
		dialog.Title, dialog.Confirm = "通过发布链接", "确认通过"
	case "reject":
		dialog.Title, dialog.Confirm = "驳回发布链接", "确认驳回"
	case "flag":
		dialog.Title, dialog.Confirm = "标记违规 / 限流", "确认处理"
	case "revoke":
		if canCall(account, "POST /assignments/:id/revoke") && row.Source == enum.SourceAssigned && len(row.Results) == 0 && row.Status != enum.AssignmentRevoked {
			dialog.Title, dialog.Confirm = "撤回这次指派？", "确定"
			return dialog
		}
		return nil
	default:
		return nil
	}

	if action == "flag" && !row.CanFlag || action != "flag" && !row.CanReview {
		return nil
	}

	var names []string
	for _, result := range row.Results {
		selectedPlatform := ctx.Query("selected_platform")
		if selectedPlatform != "" && selectedPlatform != string(result.Platform) {
			continue
		}

		if action == "flag" && result.Status == enum.SubmissionApproved && !result.Settled || action != "flag" && result.Status == enum.SubmissionPending {
			dialog.Choices = append(dialog.Choices, result)
			names = append(names, result.PlatformName)
		}
	}

	if len(dialog.Choices) == 0 {
		return nil
	}

	if len(dialog.Choices) == 1 {
		dialog.Selected = []string{dialog.Choices[0].RevisionKey}
		dialog.Choices[0].Selected = true
	}

	dialog.Names = strings.Join(names, "、")
	return dialog
}

func (admin *admin) renderReviewFragment(ctx *gin.Context, page *reviewsPage) bool {
	switch ctx.GetHeader("HX-Target") {
	case "review-drawer-slot":
		if page.Drawer != nil {
			admin.render(ctx, http.StatusOK, "review-drawer", page.Drawer)
			return true
		}
	case "review-dialog-slot":
		if page.Dialog != nil {
			admin.render(ctx, http.StatusOK, "review-dialog", page.Dialog)
			return true
		}
		if page.Person != nil {
			admin.render(ctx, http.StatusOK, "distributor-dialog", page.Person)
			return true
		}
	default:
		return false
	}

	admin.failure(ctx, http.StatusConflict, "审核记录已更新，请刷新后重试")
	return true
}

func (admin *admin) saveReview(ctx *gin.Context) {
	action := ctx.Param("action")
	permission := "POST /assignments/:id/review"
	if action == "flag" {
		permission = "POST /assignments/:id/reissue"
	} else if action == "revoke" {
		permission = "POST /assignments/:id/revoke"
	}

	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, reviewsRoute, permission) {
		return
	}

	result := "处理结果已保存"
	if action == "approve" {
		result = "审核已通过"
	} else if action == "revoke" {
		result = "已撤回，内容与当天名额已释放"
	}

	err := admin.applyReview(ctx, action)
	if err != nil {
		admin.renderReviewError(ctx, action, err)
		return
	}

	if !partialRequest(ctx) {
		redirect(ctx, reviewsRoute.Path+"?"+ctx.Request.URL.Query().Encode())
		return
	}

	data := admin.workspace(ctx, reviewsRoute)
	data.Reviews, err = admin.loadReviews(ctx)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		data.Error = message
		admin.renderWorkspace(ctx, status, data)
		return
	}

	data.Success = result
	admin.renderWorkspace(ctx, http.StatusOK, data)
}

func (admin *admin) applyReview(ctx *gin.Context, action string) error {
	request := ctx.Request.Context()
	account := currentOperator(ctx)
	id := ctx.Param("id")
	reason := strings.TrimSpace(ctx.PostForm("reason"))
	if action == "revoke" {
		return admin.services.Revoke(request, account.ID, id, reason)
	}

	if action != "approve" && action != "reject" && action != "flag" {
		return app.NotFound()
	}

	selected := ctx.PostFormArray("platforms")
	if len(selected) == 0 {
		return app.Invalid("请选择发布平台")
	}

	if reason == "" && action == "reject" {
		return app.Invalid("请填写驳回原因")
	}

	if reason == "" && action == "flag" {
		return app.Invalid("请填写处理原因")
	}

	choices := make([]submission.ReviewRevision, 0, len(selected))
	for _, key := range selected {
		platform, rawRevision, _ := strings.Cut(key, ":")
		revision, err := strconv.ParseInt(rawRevision, 10, 64)
		if err != nil || revision < 1 || !enum.Platform(platform).Valid() {
			return app.Invalid("请选择有效的发布平台")
		}

		choices = append(choices, submission.ReviewRevision{
			Platform: enum.Platform(platform),
			Revision: revision,
		})
	}

	if action == "flag" {
		if len(choices) != 1 {
			return app.Invalid("请选择一个发布平台")
		}

		_, err := admin.services.Reissue(request, account.ID, id, choices[0].Platform, reason, ctx.PostForm("keep_pay") == "1")
		return err
	}

	return admin.services.ReviewAtRevisions(request, account.ID, id, choices, action == "approve", reason)
}

func (admin *admin) renderReviewError(ctx *gin.Context, action string, actionErr error) {
	status, message := adminErrorMessage(ctx, actionErr)
	query := ctx.Request.URL.Query()
	query.Set("action", action)
	query.Set("assignment", ctx.Param("id"))
	ctx.Request.URL.RawQuery = query.Encode()
	page, err := admin.loadReviews(ctx)
	if err != nil || page.Dialog == nil {
		admin.failure(ctx, status, message)
		return
	}

	page.Dialog.Reason = ctx.PostForm("reason")
	page.Dialog.Selected = ctx.PostFormArray("platforms")
	for i := range page.Dialog.Choices {
		page.Dialog.Choices[i].Selected = slices.Contains(page.Dialog.Selected, page.Dialog.Choices[i].RevisionKey)
	}

	page.Dialog.KeepPay = ctx.PostForm("keep_pay") == "1"
	page.Dialog.Error = message
	if partialRequest(ctx) {
		ctx.Header("X-Admin-Form", "review")
		admin.render(ctx, status, "review-dialog", page.Dialog)
		return
	}

	data := admin.workspace(ctx, reviewsRoute)
	data.Reviews = page
	admin.renderWorkspace(ctx, status, data)
}
