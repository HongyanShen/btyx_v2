package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/creative"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/gin-gonic/gin"
)

var distributorsRoute = workspacePage{
	Path:       "/users/manage",
	Label:      "分发员管理",
	Permission: "/users",
	Endpoint:   "GET /users",
}

const distributorBlockPermission = "POST /users/:id/block"
const enrollmentDocumentPermission = "GET /users/:id/enrollment-documents"

type distributorsPage struct {
	Rows     []distributorView
	Search   string
	Pager    pagination
	CanBlock bool
	Dialog   *distributorDialog
}

type distributorView struct {
	identity.UserDTO
	DisplayName string
	DouyinLink  platformAccount
	XHSLink     platformAccount
	TagLabels   []string
}

type distributorDialog struct {
	Creative       []creative.WorkDTO
	ShowCreative   bool
	Profile        distributorView
	Mode           string
	CanBlock       bool
	Stale          bool
	CSRF           string
	Search         string
	Page           int
	Reason         string
	Error          string
	Revision       string
	EnrollmentDate string
	BlockDate      string
	Tasks          []distributorTask
	Unpublished    bool
}

type distributorTask struct {
	Title       string
	Description string
}

var assignmentSourceLabels = map[enum.AssignmentSource]string{
	enum.SourceClaim:    "自行领取",
	enum.SourceAssigned: "工作人员指派",
	enum.SourceReissued: "重新派发",
}

var assignmentStateLabels = map[enum.AssignmentStatus]string{
	enum.AssignmentInProgress:        "进行中",
	enum.AssignmentPendingReview:     "待审核",
	enum.AssignmentRejected:          "需修改",
	enum.AssignmentPendingSettlement: "待结算",
	enum.AssignmentSettled:           "已结算",
	enum.AssignmentRevoked:           "已撤回",
	enum.AssignmentExpired:           "已过期",
	enum.AssignmentNotPayable:        "不予结算",
}

func (admin *admin) loadDistributors(ctx *gin.Context) (*distributorsPage, error) {
	account := currentOperator(ctx)
	request := ctx.Request.Context()
	withContacts := canCall(account, enrollmentDocumentPermission)
	users, err := admin.services.Distributors(request, "", withContacts)
	if err != nil {
		return nil, err
	}

	tags, err := admin.contentTagCatalog(ctx, account)
	if err != nil {
		return nil, err
	}

	labels := make(map[string]string, len(tags))
	for _, tag := range tags {
		labels[tag.Key] = tag.Name
	}

	search := strings.TrimSpace(ctx.Query("q"))
	filtered := make([]identity.UserDTO, 0, len(users))
	for _, user := range users {
		for _, value := range []string{user.Name, user.Nickname, user.Phone, user.Douyin, user.XHS} {
			if strings.Contains(strings.ToLower(value), strings.ToLower(search)) {
				filtered = append(filtered, user)
				break
			}
		}
	}

	start, end, pager := listPageBounds(ctx, distributorsRoute.Path, len(filtered), 12)
	pager.Label = "分发员分页"
	if search != "" {
		pager.Search = "&q=" + url.QueryEscape(search)
	}

	page := &distributorsPage{
		Search:   search,
		Pager:    pager,
		CanBlock: canCall(account, distributorBlockPermission),
	}
	for _, user := range filtered[start:end] {
		page.Rows = append(page.Rows, distributorRow(user, labels))
	}

	id := ctx.Query("member")
	if id == "" {
		return page, nil
	}

	mode := label(ctx.Query("dialog"), "details")
	if mode != "details" && (!page.CanBlock || (mode != "block" && mode != "unblock")) {
		return nil, app.Forbidden("当前账号无权调整分发员状态")
	}

	profiles, err := admin.services.Distributors(request, id, withContacts)
	if err != nil {
		return nil, err
	}

	tasks, err := admin.services.TodayDistributorTasks(request, id)
	if err != nil {
		return nil, err
	}

	page.Dialog = distributorDialogView(ctx, distributorRow(profiles[0], labels), page, mode, tasks)
	if mode == "details" && canCall(account, creativeWorksRoute.Endpoint) {
		page.Dialog.ShowCreative = true
		page.Dialog.Creative, err = admin.services.Creative.Works(request, id)
		if err != nil {
			return nil, err
		}
	}

	return page, nil
}

func distributorRow(user identity.UserDTO, names map[string]string) distributorView {
	row := distributorView{
		UserDTO:     user,
		DisplayName: label(user.Name, label(user.Nickname, "分发员")),
		DouyinLink:  playerAccount(user.Douyin, enum.PlatformDouyin),
		XHSLink:     playerAccount(user.XHS, enum.PlatformXiaohongshu),
	}
	for _, key := range user.Tags {
		row.TagLabels = append(row.TagLabels, label(names[key], key))
	}

	return row
}

func distributorDialogView(ctx *gin.Context, row distributorView, page *distributorsPage, mode string, tasks []task.DistributorTaskDTO) *distributorDialog {
	dialog := &distributorDialog{
		Profile:        row,
		Mode:           mode,
		CanBlock:       page.CanBlock,
		Stale:          mode != "details" && row.Blocked == (mode == "block"),
		CSRF:           csrfToken(sessionToken(ctx)),
		Search:         page.Search,
		Page:           page.Pager.Current,
		Revision:       row.DistributorDetails.UpdatedAt.Format(time.RFC3339Nano),
		EnrollmentDate: "—",
		BlockDate:      "—",
	}
	if row.EnrollmentSubmittedAt != nil {
		dialog.EnrollmentDate = dateText(*row.EnrollmentSubmittedAt)
	}

	if row.BlockInfo != nil {
		dialog.BlockDate = dateText(row.BlockInfo.At)
	}

	for _, item := range tasks {
		dialog.Tasks = append(dialog.Tasks, distributorTask{
			Title:       item.Title,
			Description: assignmentSourceLabels[item.Source] + " · " + assignmentStateLabels[item.Status],
		})
		dialog.Unpublished = dialog.Unpublished || item.Unpublished
	}

	return dialog
}

func (admin *admin) saveDistributorBlock(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, distributorsRoute, distributorBlockPermission) {
		return
	}

	mode := ctx.PostForm("mode")
	reason := strings.TrimSpace(ctx.PostForm("reason"))
	revision, err := time.Parse(time.RFC3339Nano, ctx.PostForm("updated_at"))
	if err != nil || (mode != "block" && mode != "unblock") {
		admin.failure(ctx, http.StatusBadRequest, "分发员状态已失效，请刷新后重试")
		return
	}

	id := ctx.Param("id")
	err = admin.services.Block(ctx.Request.Context(), currentOperator(ctx).ID, id, mode == "block", reason, &revision)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		query := ctx.Request.URL.Query()
		query.Set("member", id)
		query.Set("dialog", mode)
		ctx.Request.URL.RawQuery = query.Encode()
		page, loadErr := admin.loadDistributors(ctx)
		if loadErr != nil {
			status, message = adminErrorMessage(ctx, loadErr)
			admin.failure(ctx, status, message)
			return
		}

		page.Dialog.Error = message
		page.Dialog.Reason = reason
		if partialRequest(ctx) {
			ctx.Header("X-Admin-Form", "distributor")
			admin.render(ctx, status, "distributor-dialog", page.Dialog)
			return
		}

		data := admin.workspace(ctx, distributorsRoute)
		data.Distributors = page
		admin.renderWorkspace(ctx, status, data)
		return
	}

	if !partialRequest(ctx) {
		redirect(ctx, distributorsRoute.Path+"?"+ctx.Request.URL.Query().Encode())
		return
	}

	data := admin.workspace(ctx, distributorsRoute)
	data.Distributors, err = admin.loadDistributors(ctx)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		data.Error = message
		admin.renderWorkspace(ctx, status, data)
		return
	}

	data.Success = "已取消拉黑"
	if mode == "block" {
		data.Success = "已拉黑"
	}

	admin.renderWorkspace(ctx, http.StatusOK, data)
}

func distributorLink(page int, search, id, mode string) string {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("q", search)
	query.Set("member", id)
	query.Set("dialog", mode)
	return distributorsRoute.Path + "?" + query.Encode()
}
