package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type auditRequest struct {
	Query      string            `json:"query"`
	Path       map[string]string `json:"path"`
	Body       string            `json:"body,omitempty"`
	BodyBase64 []byte            `json:"body_base64,omitempty"`
	Form       url.Values        `json:"multipart_fields,omitempty"`
}

func captureAuditRequest(ctx *gin.Context) {
	if ctx.FullPath() == "" || strings.HasPrefix(ctx.FullPath(), "/assets/") {
		return
	}

	input := auditRequest{
		Query: ctx.Request.URL.RawQuery,
		Path:  make(map[string]string, len(ctx.Params)),
	}
	for _, part := range ctx.Params {
		input.Path[part.Key] = part.Value
	}

	if err := input.readBody(ctx.Request); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Abort(ctx, http.StatusRequestEntityTooLarge, "请求内容过大")
			return
		}

		httpx.Abort(ctx, http.StatusBadRequest, "请求内容无效")
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		httpx.Abort(ctx, http.StatusInternalServerError, "无法记录操作参数")
		return
	}

	ctx.Request = ctx.Request.WithContext(app.WithAuditParameters(ctx.Request.Context(), string(data)))
}

func (input *auditRequest) readBody(request *http.Request) error {
	if request.Body == nil || request.Body == http.NoBody || request.ContentLength == 0 {
		return nil
	}

	mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	disposition, params, _ := mime.ParseMediaType(request.Header.Get("Content-Disposition"))
	if disposition == "attachment" || params["filename"] != "" {
		return nil
	}

	if mediaType == "multipart/form-data" {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			return err
		}

		input.Form = request.MultipartForm.Value
		return nil
	}

	// A standalone binary upload is left to the handler.
	if mediaType == "application/octet-stream" || strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "audio/") || strings.HasPrefix(mediaType, "video/") {
		return nil
	}

	data, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}

	if err := request.Body.Close(); err != nil {
		return err
	}

	request.Body = io.NopCloser(bytes.NewReader(data))
	textBody := strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" ||
		strings.HasSuffix(mediaType, "+json") || mediaType == "application/x-www-form-urlencoded"
	if !textBody && !strings.HasPrefix(http.DetectContentType(data), "text/") {
		return nil
	}

	if utf8.Valid(data) {
		input.Body = string(data)
	} else {
		input.BodyBase64 = data
	}

	return nil
}
