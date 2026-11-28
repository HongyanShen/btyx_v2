package main

import (
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/creative"
	"github.com/gin-gonic/gin"
)

var creativeTasksRoute = workspacePage{
	Path:     "/creative/tasks",
	Label:    "创作任务",
	Endpoint: "GET /creative/tasks",
}
var creativeWorksRoute = workspacePage{
	Path:     "/creative/works",
	Label:    "创作作品审核",
	Endpoint: "GET /creative/works",
}

type creativePage struct {
	Tasks         []creative.TaskDTO
	Works         []creative.WorkDTO
	Filter        string
	Reviewing     bool
	CanCreate     bool
	CanVisibility bool
	CanReview     bool
	CanReturn     bool
	CanAward      bool
	CanFiles      bool
	CanReadLocal  bool
	Username      string
	CanUpload     bool
}

func (admin *admin) loadCreative(ctx *gin.Context, path string) (*creativePage, error) {
	account := currentOperator(ctx)
	page := &creativePage{
		Reviewing:     path == creativeWorksRoute.Path,
		Username:      account.Username,
		CanReadLocal:  canCall(account, "GET /files/:id"),
		CanFiles:      canCall(account, "GET /files/:id/download"),
		CanUpload:     canCall(account, "POST /files/uploads") && canCall(account, "POST /files/:id/complete"),
		CanCreate:     canCall(account, "POST /creative/tasks"),
		CanVisibility: canCall(account, "POST /creative/tasks/:id/visibility"),
		CanReview:     canCall(account, "POST /creative/works/:id/review"),
		CanReturn:     canCall(account, "POST /creative/works/:id/return"),
		CanAward:      canCall(account, "POST /creative/works/:id/award"),
	}
	var err error
	if page.Reviewing {
		page.Works, err = admin.services.Creative.Works(ctx.Request.Context(), "")
	} else {
		page.Tasks, err = admin.services.Creative.Tasks(ctx.Request.Context(), false)
	}

	if err == nil && page.Reviewing {
		page.Filter = ctx.DefaultQuery("state", "pending")
		if page.Filter != "all" {
			selected := page.Works[:0]
			for _, work := range page.Works {
				if string(work.Status) == page.Filter {
					selected = append(selected, work)
				}
			}

			page.Works = selected
		}
	}

	return page, err
}

func creativeStatus(value enum.CreativeWorkStatus) string {
	switch value {
	case enum.CreativeDraft:
		return "已领取"
	case enum.CreativePending:
		return "待审作品"
	case enum.CreativeRejected:
		return "已驳回"
	case enum.CreativeApproved:
		return "可发布"
	case enum.CreativeLinked:
		return "待核点赞"
	case enum.CreativeRewarded:
		return "奖励已核定"
	}

	return "—"
}
