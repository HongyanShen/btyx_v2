package main

import (
	"baituber.cn/yanxuan/v2/ent/creativework"
	"baituber.cn/yanxuan/v2/ent/feedback"
	"baituber.cn/yanxuan/v2/ent/submission"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
	"slices"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
)

type menuDefinition struct {
	Label string
	Paths []string
}

func navigation(account identity.OperatorDTO, selected string) []navigationGroup {
	menus := []menuDefinition{
		{
			Label: "营销分发",
			Paths: []string{"/tasks", creativeTasksRoute.Path, creativeWorksRoute.Path, "/users", "/users/manage", "/reviews", "/settlements", "/withdrawals", distributionSettingsPath},
		},
		{
			Label: "报名审核",
			Paths: []string{"/admin/realname"},
		},
		{
			Label: "通用管理",
			Paths: []string{"/ledger", "/agreements", "/feedback", "/audits", "/operators"},
		},
	}
	if selected == "/contents" && canViewPage(account, workspacePages[0]) {
		selected = "/tasks"
	}

	var groups []navigationGroup
	for _, menu := range menus {
		var entries []navigationEntry
		for _, path := range menu.Paths {
			index := slices.IndexFunc(workspacePages, func(page workspacePage) bool {
				return page.Path == path
			})
			if index >= 0 && canViewPage(account, workspacePages[index]) && canCall(account, workspacePages[index].Endpoint) {
				entries = append(entries, navigationEntry{
					Label:    workspacePages[index].Label,
					Path:     path,
					Selected: path == selected,
				})
			}
		}

		if len(entries) == 0 {
			continue
		}

		groups = append(groups, navigationGroup{
			Open:    slices.ContainsFunc(entries, func(entry navigationEntry) bool { return entry.Selected }),
			Label:   menu.Label,
			Entries: entries,
		})
	}

	return groups
}

func (admin *admin) addPendingCounts(ctx *gin.Context, account identity.OperatorDTO, groups []navigationGroup) {
	request := ctx.Request.Context()
	for i, group := range groups {
		for j, entry := range group.Entries {
			var count int
			var err error
			switch entry.Path {
			case creativeWorksRoute.Path:
				if canCall(account, creativeWorksRoute.Endpoint) {
					count, err = admin.services.Client.CreativeWork.Query().Where(creativework.StatusEQ(enum.CreativePending)).Count(request)
				}
			case enrollmentsRoute.Path:
				if canCall(account, enrollmentsRoute.Endpoint) {
					count, err = admin.services.Client.UserEnrollment.Query().Where(userenrollment.StatusEQ(enum.EnrollmentPending)).Count(request)
				}
			case reviewsRoute.Path:
				if reviewsRoute.Endpoint != "" && canCall(account, reviewsRoute.Endpoint) {
					count, err = admin.services.Client.Submission.Query().Where(submission.StatusEQ(enum.SubmissionPending)).Count(request)
				}
			case feedbackRoute.Path:
				if canCall(account, feedbackRoute.Endpoint) {
					count, err = admin.services.Client.Feedback.Query().Where(feedback.StatusEQ(enum.FeedbackOpen)).Count(request)
				}
			}

			if err != nil {
				logging.Error(request, err).Msg("读取菜单待办数失败")
				continue
			}

			group.Entries[j].Pending = count
			groups[i].Pending += count
		}
	}
}
