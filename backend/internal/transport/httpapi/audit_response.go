package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

type auditResponseWriter struct {
	gin.ResponseWriter
	ctx         *gin.Context
	body        bytes.Buffer
	isFile      bool
	classified  bool
	contentType string
}

func (w *auditResponseWriter) classify(sample []byte) {
	if w.classified {
		return
	}

	w.classified = true
	w.contentType = w.Header().Get("Content-Type")
	if w.contentType == "" && len(sample) > 0 {
		w.contentType = http.DetectContentType(sample)
	}

	disposition, params, _ := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	media, _, _ := mime.ParseMediaType(w.contentType)
	w.isFile = disposition == "attachment" || params["filename"] != "" ||
		strings.HasPrefix(media, "image/") || media == "text/csv" ||
		(media != "" && !strings.HasPrefix(media, "text/") && media != "application/json" && !strings.HasSuffix(media, "+json") &&
			media != "application/xml" && !strings.HasSuffix(media, "+xml") && media != "application/x-www-form-urlencoded")
}

func failedWriteActor(ctx *gin.Context) string {
	if ctx.Writer.Status() < http.StatusBadRequest {
		return ""
	}

	switch ctx.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		value, _ := ctx.Get("principal")
		principal, _ := value.(identity.Principal)
		return principal.ID
	default:
		return ""
	}
}

func (w *auditResponseWriter) capture() bool {
	return app.AuditEventAttempted(w.ctx.Request.Context()) || failedWriteActor(w.ctx) != ""
}

func (w *auditResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 && w.capture() {
		w.classify(data[:n])
		if !w.isFile {
			w.body.Write(data[:n])
		}
	}

	return n, err
}

func (w *auditResponseWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	if n > 0 && w.capture() {
		if !w.classified {
			w.classify([]byte(data[:min(n, 512)]))
		}

		if !w.isFile {
			w.body.WriteString(data[:n])
		}
	}

	return n, err
}

func auditHTTPResponses(runtime *app.Runtime) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.FullPath() == "" || strings.HasPrefix(ctx.FullPath(), "/assets/") {
			return
		}

		writer := &auditResponseWriter{
			ResponseWriter: ctx.Writer,
			ctx:            ctx,
		}
		ctx.Writer = writer
		ctx.Next()
		if !writer.capture() {
			return
		}

		writer.classify(nil)
		response := app.AuditResponse{
			Status:      writer.Status(),
			ContentType: writer.contentType,
			IsFile:      writer.isFile,
			Body:        writer.body.String(),
			FailedActor: failedWriteActor(ctx),
			Reason:      ctx.GetString(httpx.AuditFailureMessage),
		}
		if id := ctx.Param("id"); id != "" {
			if _, err := ksuid.Parse(id); err == nil {
				response.ReferenceID = id
			}
		}

		if response.FailedActor != "" && response.Reason == "" {
			response.Reason = fmt.Sprintf("操作请求未完成（HTTP %d）", response.Status)
		}

		request, cancel := context.WithTimeout(context.WithoutCancel(ctx.Request.Context()), 3*time.Second)
		defer cancel()

		if err := runtime.SaveAuditResponse(request, response); err != nil {
			logging.Error(request, err).Msg("记录接口响应审计时出错")
		}
	}
}
