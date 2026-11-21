package httpapi

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/modules/feedback"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type feedbackHandler struct {
	service *feedback.Service
}

func (handler *feedbackHandler) feedback(ctx *gin.Context) ([]feedback.DTO, error) {
	principal := CurrentPrincipal(ctx)
	return handler.service.Feedbacks(ctx.Request.Context(), principal.ID, principal.Role)
}

func FeedbackRoutes(service *feedback.Service) []httpx.Route {
	handler := &feedbackHandler{
		service: service,
	}
	return []httpx.Route{
		{
			Method:  http.MethodGet,
			Path:    "/feedback",
			Label:   "查看反馈与咨询",
			Handler: httpx.Adapt(handler.feedback),
		},
	}
}
