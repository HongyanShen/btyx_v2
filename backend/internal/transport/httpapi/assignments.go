package httpapi

import (
	"net/http"
	"strconv"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type assignmentHandler struct {
	service *task.Service
}

func AssignmentRoutes(service *task.Service) []httpx.Route {
	handler := &assignmentHandler{
		service: service,
	}
	return []httpx.Route{
		{
			Method:  http.MethodGet,
			Path:    "/assignments",
			Label:   "查看任务执行列表",
			Handler: httpx.Adapt(handler.listAssignments),
		},
		{
			Method:  http.MethodGet,
			Path:    "/assignments/:id",
			Label:   "查看任务执行详情",
			Handler: httpx.Adapt(handler.getAssignment),
		},
	}
}

func (handler *assignmentHandler) listAssignments(ctx *gin.Context) ([]task.AssignmentDTO, error) {
	principal := CurrentPrincipal(ctx)
	if value, paged := ctx.GetQuery("page"); paged && principal.Role == enum.RoleAdmin {
		page, err := strconv.Atoi(value)
		if err != nil {
			return nil, app.Invalid("分页参数无效")
		}

		rows, total, err := handler.service.ReviewPage(ctx.Request.Context(), page,
			enum.AssignmentStatus(ctx.Query("status")), enum.Platform(ctx.Query("platform")),
		)
		if err != nil {
			return nil, err
		}

		ctx.Header("X-Total-Count", strconv.Itoa(total))
		return rows, nil
	}

	return handler.service.Assignments(ctx.Request.Context(), principal.ID, principal.Role)
}

func (handler *assignmentHandler) getAssignment(ctx *gin.Context) (task.AssignmentDTO, error) {
	principal := CurrentPrincipal(ctx)
	if principal.Role == enum.RoleAdmin {
		return handler.service.AdminAssignment(ctx.Request.Context(), ctx.Param("id"))
	}

	return handler.service.Assignment(ctx.Request.Context(), principal.ID, ctx.Param("id"))
}
