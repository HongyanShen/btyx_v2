package httpapi

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type fileHandler struct {
	service *files.Service
	role    enum.Role
}

func (handler *fileHandler) initiateUpload(ctx *gin.Context, input files.InitiateInput) (files.UploadTicket, error) {
	ctx.Header("Cache-Control", "private, no-store")

	return handler.service.InitiateUpload(
		ctx.Request.Context(), CurrentUser(ctx), handler.role, input, ctx.GetHeader("Idempotency-Key"),
	)
}

func (handler *fileHandler) completeUpload(ctx *gin.Context) (files.DTO, error) {
	return handler.service.CompleteUpload(ctx.Request.Context(), CurrentUser(ctx), ctx.Param("id"))
}

func (handler *fileHandler) download(ctx *gin.Context) (files.DownloadDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")

	return handler.service.Download(ctx.Request.Context(), CurrentUser(ctx), handler.role, ctx.Param("id"))
}

func (handler *fileHandler) readFile(ctx *gin.Context) (httpx.FileResponse, error) {
	record, err := handler.service.FileForRead(ctx.Request.Context(), CurrentUser(ctx), handler.role, ctx.Param("id"))
	if err != nil {
		return httpx.FileResponse{}, err
	}

	if record.Storage != enum.FileStorageLocal {
		return httpx.FileResponse{}, app.Conflict("请先获取该文件的短期下载地址")
	}

	return httpx.FileResponse{
		Path:      record.Path,
		Name:      record.Name,
		MediaType: string(record.MediaType),
	}, nil
}

func FileRoutes(service *files.Service, role enum.Role) []httpx.Route {
	handler := &fileHandler{
		service: service,
		role:    role,
	}
	return []httpx.Route{
		{
			Method:  http.MethodPost,
			Path:    "/files/uploads",
			Label:   "申请附件上传凭证",
			Handler: httpx.JSON(handler.initiateUpload),
		},
		{
			Method:  http.MethodPost,
			Path:    "/files/:id/complete",
			Label:   "确认附件上传",
			Handler: httpx.Adapt(handler.completeUpload),
		},
		{
			Method:  http.MethodGet,
			Path:    "/files/:id/download",
			Label:   "获取附件下载凭证",
			Handler: httpx.Adapt(handler.download),
		},
		{
			Method:  http.MethodGet,
			Path:    "/files/:id",
			Label:   "下载历史本地附件",
			Handler: httpx.Adapt(handler.readFile),
		},
	}
}
