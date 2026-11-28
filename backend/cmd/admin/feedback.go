package main

import (
	"net/http"
	"net/url"
	"slices"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/feedback"
	"github.com/gin-gonic/gin"
)

var feedbackRoute = workspacePage{
	Path:     "/feedback",
	Label:    "反馈与咨询",
	Endpoint: "GET /feedback",
}

const replyPermission = "POST /feedback/:id/reply"

type feedbackRow struct {
	feedback.DTO
	Name  string
	Label string
	Date  string
}

type feedbackPage struct {
	Pager    pagination
	Rows     []feedbackRow
	CanReply bool
	Dialog   *feedbackDialog
}

type feedbackDialog struct {
	Record   feedback.DTO
	CanReply bool
	CSRF     string
	Page     int
	Error    string
}

func (admin *admin) loadFeedback(ctx *gin.Context) (*feedbackPage, error) {
	request := ctx.Request.Context()
	account := currentOperator(ctx)
	records, err := admin.services.Feedbacks(request, account.ID, enum.RoleAdmin)
	if err != nil {
		return nil, err
	}

	names := make(map[string]string)
	if len(records) > 0 && canCall(account, "GET /users") {
		users, err := admin.services.Users(request)
		if err != nil {
			return nil, err
		}

		for _, user := range users {
			names[user.ID] = label(user.Name, user.Douyin)
		}
	}

	start, end, pager := pageBounds(ctx, "/feedback", len(records), 10)
	pager.Legacy = true
	page := &feedbackPage{
		Pager:    pager,
		CanReply: canCall(account, replyPermission),
	}
	for _, record := range records[start:end] {
		page.Rows = append(page.Rows, feedbackRow{
			DTO:   record,
			Name:  label(names[record.UserID], "分发员"),
			Label: label(feedbackLabels[record.Status], string(record.Status)),
			Date:  dateText(record.CreatedAt),
		})
	}

	return page, nil
}

func (admin *admin) feedbackDetail(ctx *gin.Context) {
	dialog, ok := admin.loadFeedbackDialog(ctx)
	if ok {
		admin.renderFeedbackDialog(ctx, http.StatusOK, dialog)
	}
}

func (admin *admin) feedbackReply(ctx *gin.Context) {
	if !admin.allowForm(ctx) {
		return
	}

	dialog, ok := admin.loadFeedbackDialog(ctx)
	if !ok {
		return
	}

	if !dialog.CanReply {
		admin.failure(ctx, http.StatusForbidden, "当前账号无权回复反馈")

		return
	}

	reply := ctx.PostForm("reply")
	err := admin.services.ReplyFeedback(ctx.Request.Context(), currentOperator(ctx).ID, dialog.Record.ID, reply)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		dialog.Error = message
		dialog.Record.Reply = reply
		admin.renderFeedbackDialog(ctx, status, dialog)

		return
	}

	if !partialRequest(ctx) {
		redirect(ctx, "/feedback?page="+url.QueryEscape(ctx.Query("page")))

		return
	}

	data := admin.workspace(ctx, feedbackRoute)
	data.Feedback, err = admin.loadFeedback(ctx)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		data.Error = message
		admin.renderWorkspace(ctx, status, data)

		return
	}

	data.Success = "已回复，分发员可在小程序查看"
	admin.renderWorkspace(ctx, http.StatusOK, data)
}

func (admin *admin) loadFeedbackDialog(ctx *gin.Context) (feedbackDialog, bool) {
	account := currentOperator(ctx)
	if !canViewPage(account, feedbackRoute) || !canCall(account, feedbackRoute.Endpoint) {
		admin.failure(ctx, http.StatusForbidden, "当前账号无权查看反馈")

		return feedbackDialog{}, false
	}

	records, err := admin.services.Feedbacks(ctx.Request.Context(), account.ID, enum.RoleAdmin)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		admin.failure(ctx, status, message)

		return feedbackDialog{}, false
	}

	id := ctx.Param("id")
	index := slices.IndexFunc(records, func(record feedback.DTO) bool {
		return record.ID == id
	})
	if index == -1 {
		admin.failure(ctx, http.StatusNotFound, "反馈记录不存在，请刷新列表")

		return feedbackDialog{}, false
	}

	_, _, pager := pageBounds(ctx, feedbackRoute.Path, len(records), 10)

	return feedbackDialog{
		Record:   records[index],
		CanReply: canCall(account, replyPermission),
		CSRF:     csrfToken(sessionToken(ctx)),
		Page:     pager.Current,
	}, true
}

func (admin *admin) renderFeedbackDialog(ctx *gin.Context, status int, dialog feedbackDialog) {
	if partialRequest(ctx) {
		ctx.Header("X-Admin-Form", "feedback")
		admin.render(ctx, status, "feedback-dialog", dialog)

		return
	}

	page, err := admin.loadFeedback(ctx)
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		admin.failure(ctx, status, message)

		return
	}

	data := admin.workspace(ctx, feedbackRoute)
	page.Dialog = &dialog
	data.Feedback = page
	admin.renderWorkspace(ctx, status, data)
}

var feedbackLabels = map[enum.FeedbackStatus]string{
	enum.FeedbackOpen:    "待处理",
	enum.FeedbackReplied: "已回复",
	enum.FeedbackClosed:  "已关闭",
}
