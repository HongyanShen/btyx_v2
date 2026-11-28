package operations

import (
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type revokeAssignmentInput struct {
	Reason string `json:"reason"`
}

type reissueAssignmentInput struct {
	Platform enum.Platform `json:"platform"`
	Reason   string        `json:"reason"`
	KeepPay  bool          `json:"keep_pay"`
}

type reviewSubmissionInput struct {
	submission.ReviewInput
	Submissions []submission.ReviewRevision `json:"submissions"`
}

func (handler *Handler) revokeAssignment(ctx *gin.Context, input revokeAssignmentInput) (*httpx.NoData, error) {
	return nil, handler.services.Revoke(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input.Reason)
}

func (handler *Handler) reviewSubmission(ctx *gin.Context, input reviewSubmissionInput) (*httpx.NoData, error) {
	request := ctx.Request.Context()
	operatorID := httpapi.CurrentUser(ctx)
	assignmentID := ctx.Param("id")
	if input.Submissions != nil {
		if input.Platform != "" || input.Revision != 0 {
			return nil, app.Invalid("批量审核请在平台列表中提供各平台的提交版本")
		}

		return nil, handler.services.ReviewAtRevisions(request, operatorID, assignmentID, input.Submissions, input.Approved, input.Reason)
	}

	return nil, handler.services.ReviewAtRevision(request, operatorID, assignmentID, input.ReviewInput)
}

func (handler *Handler) reissueAssignment(ctx *gin.Context, input reissueAssignmentInput) (task.AssignmentDTO, error) {
	return handler.services.Reissue(
		ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"),
		input.Platform, input.Reason, input.KeepPay,
	)
}
