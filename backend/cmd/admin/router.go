package main

import (
	"embed"
	"github.com/shopspring/decimal"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/cmd/admin/operations"
	"baituber.cn/yanxuan/v2/internal/business"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

//go:embed templates/*/*.html
var pagesFS embed.FS

//go:embed static
var staticFS embed.FS

type admin struct {
	services      *business.Service
	templates     *template.Template
	catalog       operations.OperatorCatalog
	secureCookies bool
	origin        http.CrossOriginProtection
}

func newAdminRouter(services *business.Service, secureCookies bool) (*gin.Engine, error) {
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}

	templates, err := template.New("admin").Funcs(template.FuncMap{
		"previousPage":     func(page int) int { return page - 1 },
		"nextPage":         func(page int) int { return page + 1 },
		"creativeTime":     func(t time.Time) string { return t.In(beijing).Format("2006-01-02 15:04") },
		"creativeStatus":   creativeStatus,
		"creativePlatform": func(p enum.Platform) string { return platformNames[p] },
		"creativeFen":      func(fen string) string { v, _ := decimal.NewFromString(fen); return v.Shift(-2).StringFixed(2) },
		"distributorLink":  distributorLink,
		"enrollmentLink":   enrollmentLink,
		"reviewLink":       reviewLink,
		"queryEscape":      url.QueryEscape,
		"watermarkTiles":   func() [48]struct{} { return [48]struct{}{} },
		"settlementLink":   settlementLink,
		"marketingLink":    marketingLink,
		"textBoundary":     func() template.HTML { return "<!---->" },
		"shortUploadDate":  func(date string) string { return strings.ReplaceAll(date[5:], "-", "/") },
	}).ParseFS(pagesFS, "templates/*/*.html")
	if err != nil {
		return nil, err
	}

	admin := &admin{
		services:      services,
		templates:     templates,
		secureCookies: secureCookies,
	}
	router := httpapi.NewRouter(services.Runtime, enum.AuditPlatformAdmin)
	router.RedirectTrailingSlash = false
	router.Use(admin.securityHeaders())
	router.StaticFS("/assets", http.FS(assets))
	admin.catalog = operations.Register(router, services, admin.apiAuth(), assignablePages(workspacePages))

	router.GET("/login", admin.loginPage)
	router.POST("/login", admin.login)
	protected := router.Group("", admin.requireSession)
	protected.GET("/", admin.home)
	protected.GET("/profile", admin.profilePage)
	protected.POST("/profile/password", admin.changeProfilePassword)
	protected.POST("/logout", admin.logout)
	protected.GET("/feedback/:id/reply", admin.feedbackDetail)
	protected.POST("/feedback/:id/reply", admin.feedbackReply)
	protected.POST("/ledger/:id/adjust", admin.ledgerAdjust)
	protected.POST("/agreements", admin.agreementPublish)
	protected.POST("/operators", admin.saveOperator)
	protected.POST("/operators/:id", admin.saveOperator)
	protected.POST("/contents", admin.saveContent)
	protected.POST("/contents/:id/tags", admin.saveContent)
	protected.POST("/users/manage/:id/block", admin.saveDistributorBlock)
	protected.POST("/users/:id/review", admin.saveEnrollment)
	protected.POST("/users/tags", admin.updateEnrollmentTag)
	protected.POST("/users/tags/:id/:action", admin.updateEnrollmentTag)
	protected.POST("/reviews/:id/:action", admin.saveReview)
	protected.POST("/settlements", admin.confirmSettlement)
	protected.POST(distributionSettingsPath+"/marketing", admin.saveDispatchControls)
	protected.POST(distributionSettingsPath, admin.saveDistributionSettings)
	protected.POST("/tasks", admin.saveMarketingTask)
	protected.POST("/tasks/:id/:action", admin.saveMarketingTask)
	protected.POST("/tasks/settings", admin.saveMarketingSettings)

	for _, page := range workspacePages {
		protected.GET(page.Path, admin.page(page))
	}

	router.NoRoute(func(ctx *gin.Context) {
		admin.render(ctx, http.StatusNotFound, "error-document", adminError{
			Status:  http.StatusNotFound,
			Message: "页面不存在",
		})
	})

	return router, nil
}

func (admin *admin) securityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Referrer-Policy", "no-referrer")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; media-src 'self' blob: https:; connect-src 'self' https:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		ctx.Header("Cache-Control", "private, no-store")
		ctx.Writer.Header().Add("Vary", "HX-Request, HX-History-Restore-Request, HX-Target")
		ctx.Next()
	}
}

func (admin *admin) apiAuth() gin.HandlerFunc {
	authenticate := httpapi.AuthenticateWithToken(admin.services.IdentityService, enum.RoleAdmin, sessionToken)

	return func(ctx *gin.Context) {
		if !admin.allowAPIRequest(ctx) {
			return
		}

		authenticate(ctx)
	}
}
