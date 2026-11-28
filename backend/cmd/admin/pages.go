package main

import (
	"bytes"
	"errors"
	"net/http"
	"slices"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
)

type workspacePage struct {
	Path            string
	Label           string
	Permission      string
	PermissionLabel string
	Endpoint        string
}

func assignablePages(pages []workspacePage) []identity.OperatorPage {
	result := make([]identity.OperatorPage, 0, len(pages))
	for _, page := range pages {
		if page.Path == operatorsRoute.Path || page.Permission != "" {
			continue
		}

		label := page.PermissionLabel
		if label == "" {
			label = page.Label
		}

		result = append(result, identity.OperatorPage{
			Path:  page.Path,
			Label: label,
		})
	}

	return result
}

var workspacePages = []workspacePage{
	tasksRoute,
	creativeTasksRoute,
	creativeWorksRoute,
	contentRoute,
	enrollmentsRoute,
	distributorsRoute,
	reviewsRoute,
	settlementsRoute,
	distributionSettingsRoute,
	{
		Path:     "/withdrawals",
		Label:    "提现记录",
		Endpoint: "GET /withdrawals",
	},
	{
		Path:     identityRecordsPath,
		Label:    "实名认证记录",
		Endpoint: "GET /identity-verifications",
	},
	ledgerRoute,
	agreementsRoute,
	feedbackRoute,
	{
		Path:     "/audits",
		Label:    "操作审计",
		Endpoint: "GET /audits",
	},
	operatorsRoute,
}

type navigationEntry struct {
	Label    string
	Path     string
	Selected bool
	Pending  int
}

type navigationGroup struct {
	Pending int
	Open    bool
	Label   string
	Entries []navigationEntry
}

type workspaceData struct {
	Profile      *operatorProfilePage
	Title        string
	Path         string
	User         string
	CSRF         string
	Nav          []navigationGroup
	Error        string
	Success      string
	Audits       *auditPage
	Withdrawals  *withdrawalPage
	Feedback     *feedbackPage
	Identity     *identityRecordsPage
	Ledger       *ledgerPage
	Agreements   *agreementsPage
	Operators    *operatorsPage
	Content      *contentPage
	Distributors *distributorsPage
	Enrollments  *enrollmentsPage
	Reviews      *reviewsPage
	Settlement   *settlementPage
	Distribution *distributionSettingsPage
	Creative     *creativePage
	Tasks        *marketingPage
}

type adminError struct {
	Status  int
	Message string
}

func partialRequest(ctx *gin.Context) bool {
	return ctx.GetHeader("HX-Request") == "true" && ctx.GetHeader("HX-History-Restore-Request") != "true"
}

func (admin *admin) render(ctx *gin.Context, status int, name string, model any) {
	var output bytes.Buffer
	if err := admin.templates.ExecuteTemplate(&output, name, model); err != nil {
		logging.Error(ctx.Request.Context(), err).Msg("admin template rendering failed")
		ctx.String(http.StatusInternalServerError, "页面暂时无法显示，请稍后重试")

		return
	}

	ctx.Data(status, "text/html; charset=utf-8", output.Bytes())
}

func (admin *admin) failure(ctx *gin.Context, status int, message string) {
	ctx.Set(httpx.AuditFailureMessage, message)
	data := adminError{
		Status:  status,
		Message: message,
	}
	if partialRequest(ctx) {
		admin.render(ctx, status, "error-message", data)

		return
	}

	admin.render(ctx, status, "error-document", data)
}

func adminErrorMessage(ctx *gin.Context, err error) (int, string) {
	var businessError *app.Error
	if errors.As(err, &businessError) {
		ctx.Set(httpx.AuditFailureMessage, businessError.Message)
		return businessError.Status, businessError.Message
	}

	logging.Error(ctx.Request.Context(), err).Msg("admin request failed")

	return http.StatusInternalServerError, "服务暂时不可用，请稍后重试"
}

func (admin *admin) home(ctx *gin.Context) {
	account := currentOperator(ctx)
	for _, page := range workspacePages {
		if page.Path != "/contents" && canViewPage(account, page) && canCall(account, page.Endpoint) {
			redirect(ctx, page.Path)

			return
		}
	}

	for _, page := range workspacePages {
		if canViewPage(account, page) && canCall(account, page.Endpoint) {
			redirect(ctx, page.Path)

			return
		}
	}

	data := admin.workspace(ctx, workspacePage{Label: "运营后台"})
	data.Error = "当前账号尚未分配可访问的页面权限"
	admin.renderWorkspace(ctx, http.StatusOK, data)
}

func canViewPage(account identity.OperatorDTO, page workspacePage) bool {
	if account.SuperAdmin {
		return true
	}

	permission := page.Permission
	if permission == "" {
		permission = page.Path
	}

	return permission != "/operators" && slices.Contains(account.Pages, permission)
}

func canCall(account identity.OperatorDTO, key string) bool {
	return key == "" || account.SuperAdmin || slices.Contains(account.Endpoints, key)
}

func (admin *admin) page(page workspacePage) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		data := admin.workspace(ctx, page)
		account := currentOperator(ctx)
		if !canViewPage(account, page) || !canCall(account, page.Endpoint) {
			data.Error = "无权访问此页面"
			if canViewPage(account, page) {
				data.Error = "无权读取此页面的数据"
			}

			admin.renderWorkspace(ctx, http.StatusForbidden, data)

			return
		}

		var err error
		switch page.Path {
		case "/audits":
			data.Audits, err = admin.loadAudits(ctx)
			if data.Audits != nil && data.Audits.Error != "" {
				admin.renderWorkspace(ctx, http.StatusBadRequest, data)
				return
			}
		case "/withdrawals":
			data.Withdrawals, err = admin.loadWithdrawals(ctx)
		case "/feedback":
			data.Feedback, err = admin.loadFeedback(ctx)
		case identityRecordsPath:
			data.Identity, err = admin.loadIdentityRecords(ctx, account)
		case ledgerRoute.Path:
			data.Ledger, err = admin.loadLedger(ctx)
		case agreementsRoute.Path:
			data.Agreements, err = admin.loadAgreements(ctx)
		case operatorsRoute.Path:
			data.Operators, err = admin.loadOperators(ctx)
		case contentRoute.Path:
			data.Content, err = admin.loadContent(ctx)
		case distributorsRoute.Path:
			data.Distributors, err = admin.loadDistributors(ctx)
		case enrollmentsRoute.Path:
			data.Enrollments, err = admin.loadEnrollments(ctx)
		case reviewsRoute.Path:
			data.Reviews, err = admin.loadReviews(ctx)
		case settlementsRoute.Path:
			data.Settlement, err = admin.loadSettlement(ctx)
		case distributionSettingsPath:
			data.Distribution, err = admin.loadDistributionSettings(ctx)
		case creativeTasksRoute.Path, creativeWorksRoute.Path:
			data.Creative, err = admin.loadCreative(ctx, page.Path)
		case tasksRoute.Path:
			data.Tasks, err = admin.loadMarketing(ctx)
		}

		if err != nil {
			status, message := adminErrorMessage(ctx, err)
			data.Error = message
			admin.renderWorkspace(ctx, status, data)

			return
		}

		if data.Withdrawals != nil && partialRequest(ctx) && ctx.GetHeader("HX-Target") == "withdrawal-results" {
			admin.render(ctx, http.StatusOK, "withdrawal-results", data.Withdrawals)

			return
		}

		if data.Ledger != nil && partialRequest(ctx) && ctx.GetHeader("HX-Target") == "ledger-dialog-slot" {
			if data.Ledger.Dialog == nil {
				admin.failure(ctx, http.StatusNotFound, "无法调整此流水，请刷新列表")
				return
			}

			admin.render(ctx, http.StatusOK, "ledger-dialog", data.Ledger.Dialog)
			return
		}

		if data.Agreements != nil && partialRequest(ctx) && ctx.GetHeader("HX-Target") == "agreement-dialog-slot" {
			if data.Agreements.Dialog == nil {
				admin.failure(ctx, http.StatusForbidden, "当前账号无权发布协议")
				return
			}

			admin.render(ctx, http.StatusOK, "agreement-dialog", data.Agreements.Dialog)
			return
		}

		if data.Operators != nil && partialRequest(ctx) && ctx.GetHeader("HX-Target") == "operator-dialog-slot" {
			if data.Operators.Dialog == nil {
				admin.failure(ctx, http.StatusNotFound, "运营账号不存在或不可编辑")
				return
			}

			admin.render(ctx, http.StatusOK, "operator-dialog", data.Operators.Dialog)
			return
		}

		if data.Content != nil && partialRequest(ctx) && ctx.GetHeader("HX-Target") == "content-dialog-slot" {
			if data.Content.Dialog == nil {
				admin.failure(ctx, http.StatusNotFound, "素材不存在或不可编辑")
				return
			}

			admin.render(ctx, http.StatusOK, "content-dialog", data.Content.Dialog)
			return
		}

		if data.Distributors != nil && partialRequest(ctx) {
			switch ctx.GetHeader("HX-Target") {
			case "distributor-results":
				admin.render(ctx, http.StatusOK, "distributor-results", data.Distributors)
				return
			case "distributor-dialog-slot":
				if data.Distributors.Dialog == nil {
					admin.failure(ctx, http.StatusNotFound, "分发员详情暂不可用，请刷新后重试")
					return
				}
				admin.render(ctx, http.StatusOK, "distributor-dialog", data.Distributors.Dialog)
				return
			}
		}

		enrollmentTarget := ctx.GetHeader("HX-Target")
		if data.Enrollments != nil && partialRequest(ctx) && (enrollmentTarget == "enrollment-dialog-slot" || enrollmentTarget == "enrollment-photo-slot") {
			if data.Enrollments.Dialog == nil {
				admin.failure(ctx, http.StatusNotFound, "报名记录或标签已更新，请刷新重试")
				return
			}

			admin.render(ctx, http.StatusOK, "enrollment-dialog", data.Enrollments.Dialog)
			return
		}

		if data.Reviews != nil && partialRequest(ctx) {
			if admin.renderReviewFragment(ctx, data.Reviews) {
				return
			}
		}

		if data.Settlement != nil && partialRequest(ctx) && ctx.GetHeader("HX-Target") == "settlement-dialog-slot" {
			admin.renderSettlementDialog(ctx, http.StatusOK, data.Settlement)
			return
		}

		if data.Tasks != nil && partialRequest(ctx) {
			switch ctx.GetHeader("HX-Target") {
			case "marketing-dialog-slot":
				if data.Tasks.Dialog == nil {
					ctx.Status(http.StatusNoContent)
					return
				}
				admin.render(ctx, http.StatusOK, "marketing-dialog", data.Tasks.Dialog)
				return
			case "marketing-pool-results":
				admin.render(ctx, http.StatusOK, "marketing-pool-results", data.Tasks)
				return
			}
		}

		admin.renderWorkspace(ctx, http.StatusOK, data)
	}
}

func (admin *admin) allowPageAction(ctx *gin.Context, page workspacePage, endpoint string) bool {
	account := currentOperator(ctx)
	if canViewPage(account, page) && canCall(account, page.Endpoint) && canCall(account, endpoint) {
		return true
	}

	admin.failure(ctx, http.StatusForbidden, "当前账号无权执行此操作")
	return false
}

func (admin *admin) workspace(ctx *gin.Context, page workspacePage) workspaceData {
	account := currentOperator(ctx)

	groups := navigation(account, page.Path)
	admin.addPendingCounts(ctx, account, groups)
	return workspaceData{
		Title: page.Label,
		Path:  page.Path,
		User:  account.Username,
		CSRF:  csrfToken(sessionToken(ctx)),
		Nav:   groups,
	}
}

func (admin *admin) renderWorkspace(ctx *gin.Context, status int, data workspaceData) {
	if partialRequest(ctx) {
		ctx.Header("HX-Retarget", "#admin-workspace")
		ctx.Header("HX-Reswap", "outerHTML")
		admin.render(ctx, status, "workspace", data)

		return
	}

	admin.render(ctx, status, "workspace-document", data)
}
