package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const sessionCookie = "btyx_admin_web"

type loginData struct {
	PasswordChanged bool
	Username        string
	Next            string
	Error           string
}

func sessionToken(ctx *gin.Context) string {
	cookie, _ := ctx.Request.Cookie(sessionCookie)
	if cookie == nil {
		return ""
	}

	return cookie.Value
}

func csrfToken(session string) string {
	mac := hmac.New(sha256.New, []byte(session))
	mac.Write([]byte("btyx-admin-form-csrf"))

	return hex.EncodeToString(mac.Sum(nil))
}

func validCSRF(session, provided string) bool {
	return session != "" && hmac.Equal([]byte(csrfToken(session)), []byte(provided))
}

func (admin *admin) setSession(ctx *gin.Context, token string, maxAge int) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   admin.secureCookies || ctx.Request.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func (admin *admin) loginPage(ctx *gin.Context) {
	admin.renderLogin(ctx, http.StatusOK, loginData{
		Next:            nextPage(ctx.Query("next")),
		PasswordChanged: ctx.Query("password_changed") == "1",
	})
}

func (admin *admin) login(ctx *gin.Context) {
	if admin.origin.Check(ctx.Request) != nil {
		admin.failure(ctx, http.StatusForbidden, "请从当前站点提交登录表单")

		return
	}

	data := loginData{
		Next:     nextPage(ctx.PostForm("next")),
		Username: ctx.PostForm("username"),
	}
	result, err := admin.services.LoginAdmin(ctx.Request.Context(), data.Username, ctx.PostForm("password"))
	if err != nil {
		status, message := adminErrorMessage(ctx, err)
		data.Error = message
		admin.renderLogin(ctx, status, data)

		return
	}

	admin.setSession(ctx, result.Token, int((24 * time.Hour).Seconds()))
	redirect(ctx, data.Next)
}

func (admin *admin) renderLogin(ctx *gin.Context, status int, data loginData) {
	if partialRequest(ctx) {
		ctx.Header("X-Admin-Form", "login")
		admin.render(ctx, status, "login-form", data)

		return
	}

	admin.render(ctx, status, "login-document", data)
}

func (admin *admin) requireSession(ctx *gin.Context) {
	token := sessionToken(ctx)
	if token == "" {
		redirectToLogin(ctx)
		ctx.Abort()

		return
	}

	principal, err := admin.services.Authenticate(ctx.Request.Context(), token)
	if err != nil {
		var rejected *app.Error
		if errors.As(err, &rejected) && rejected.Status == http.StatusUnauthorized {
			admin.setSession(ctx, "", -1)
			redirectToLogin(ctx)
		} else {
			status, message := adminErrorMessage(ctx, err)
			admin.failure(ctx, status, message)
		}

		ctx.Abort()

		return
	}

	if principal.Role != enum.RoleAdmin {
		admin.setSession(ctx, "", -1)
		redirectToLogin(ctx)
		ctx.Abort()

		return
	}

	ctx.Set("principal", principal)
	ctx.Set("operator", admin.catalog.EffectiveGrants(principal.Operator))
	ctx.Next()
}

func currentOperator(ctx *gin.Context) identity.OperatorDTO {
	return ctx.MustGet("operator").(identity.OperatorDTO)
}

func (admin *admin) logout(ctx *gin.Context) {
	if !admin.allowForm(ctx) {
		return
	}

	if err := admin.services.LogoutAdmin(ctx.Request.Context(), sessionToken(ctx)); err != nil {
		status, message := adminErrorMessage(ctx, err)
		admin.failure(ctx, status, message)

		return
	}

	admin.setSession(ctx, "", -1)
	redirect(ctx, "/login")
}

func (admin *admin) allowForm(ctx *gin.Context) bool {
	if admin.origin.Check(ctx.Request) == nil && validCSRF(sessionToken(ctx), ctx.PostForm("csrf")) {
		return true
	}

	admin.failure(ctx, http.StatusForbidden, "表单已失效，请刷新页面后重试")

	return false
}

func (admin *admin) allowAPIRequest(ctx *gin.Context) bool {
	if ctx.Request.Method == http.MethodGet || ctx.Request.Method == http.MethodHead {
		return true
	}

	session := sessionToken(ctx)
	if admin.origin.Check(ctx.Request) == nil && (session == "" || validCSRF(session, ctx.GetHeader("X-CSRF-Token"))) {
		return true
	}

	httpx.Abort(ctx, http.StatusForbidden, "请求校验失败，请刷新页面后重试")

	return false
}

func nextPage(raw string) string {
	path, err := url.ParseRequestURI(raw)
	if err == nil && strings.HasPrefix(raw, "/") && !path.IsAbs() && path.Host == "" {
		if path.EscapedPath() == "/profile" {
			return "/profile"
		}

		for _, page := range workspacePages {
			if path.Path == page.Path && path.EscapedPath() == page.Path {
				return path.RequestURI()
			}
		}
	}

	return "/"
}

func redirectToLogin(ctx *gin.Context) {
	next := nextPage(ctx.Request.URL.RequestURI())
	redirect(ctx, "/login?next="+url.QueryEscape(next))
}

func redirect(ctx *gin.Context, path string) {
	if partialRequest(ctx) {
		ctx.Header("HX-Redirect", path)
		ctx.Status(http.StatusOK)

		return
	}

	ctx.Redirect(http.StatusSeeOther, path)
}
