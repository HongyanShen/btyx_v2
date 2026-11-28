package main

import (
	"html/template"
	"net/http"

	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"github.com/gin-gonic/gin"
)

var agreementsRoute = workspacePage{
	Path:     "/agreements",
	Label:    "分发协议",
	Endpoint: "GET /agreements",
}

const publishAgreementPermission = "POST /agreements"

type agreementsPage struct {
	Rows       []agreementRow
	Pager      pagination
	CanPublish bool
	Dialog     *agreementDialog
}

type agreementRow struct {
	agreement.DTO
	Date string
	HTML template.HTML
}

type agreementDialog struct {
	CSRF  string
	Title string
	Body  string
	Error string
}

func (admin *admin) loadAgreements(ctx *gin.Context) (*agreementsPage, error) {
	rows, err := admin.services.Agreements(ctx.Request.Context())
	if err != nil {
		return nil, err
	}

	start, end, pager := listPageBounds(ctx, agreementsRoute.Path, len(rows), 10)
	page := &agreementsPage{
		Pager:      pager,
		CanPublish: canCall(currentOperator(ctx), publishAgreementPermission),
	}
	for _, row := range rows[start:end] {
		page.Rows = append(page.Rows, agreementRow{
			DTO:  row,
			Date: dateText(row.CreatedAt),
			HTML: template.HTML(agreement.ValidatedHTML(row.BodyHTML)),
		})
	}

	if ctx.Query("publish") == "1" && page.CanPublish {
		page.Dialog = &agreementDialog{CSRF: csrfToken(sessionToken(ctx))}
	}

	return page, nil
}

func (admin *admin) agreementPublish(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, agreementsRoute, publishAgreementPermission) {
		return
	}

	dialog := agreementDialog{
		CSRF:  csrfToken(sessionToken(ctx)),
		Title: ctx.PostForm("title"),
		Body:  ctx.PostForm("body_html"),
	}
	err := admin.services.PublishRichAgreement(ctx.Request.Context(), currentOperator(ctx).ID, dialog.Title, dialog.Body)
	status := http.StatusOK
	if err != nil {
		status, dialog.Error = adminErrorMessage(ctx, err)
		if partialRequest(ctx) {
			ctx.Header("X-Admin-Form", "agreement")
			admin.render(ctx, status, "agreement-dialog", dialog)
			return
		}
	} else if !partialRequest(ctx) {
		redirect(ctx, agreementsRoute.Path)
		return
	}

	data := admin.workspace(ctx, agreementsRoute)
	data.Agreements, err = admin.loadAgreements(ctx)
	if err != nil {
		status, data.Error = adminErrorMessage(ctx, err)
	} else if dialog.Error != "" {
		data.Agreements.Dialog = &dialog
	} else {
		data.Success = "新协议已发布"
	}

	admin.renderWorkspace(ctx, status, data)
}
