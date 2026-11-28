package main

import (
	"net/http"
	"slices"
	"strconv"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"github.com/gin-gonic/gin"
)

var operatorsRoute = workspacePage{
	Path:  "/operators",
	Label: "运营账号",
}

type operatorsPage struct {
	Rows   []identity.OperatorDTO
	Pager  pagination
	Dialog *operatorDialog
}

type operatorDialog struct {
	ID           string
	Username     string
	Disabled     bool
	AllPages     bool
	AllEndpoints bool
	CSRF         string
	Page         int
	Error        string
	Pages        []operatorChoice
	Endpoints    []operatorChoice
}

type operatorChoice struct {
	Key     string
	Label   string
	Checked bool
}

func (admin *admin) loadOperators(ctx *gin.Context) (*operatorsPage, error) {
	rows, err := admin.services.Operators(ctx.Request.Context())
	if err != nil {
		return nil, err
	}

	start, end, pager := listPageBounds(ctx, operatorsRoute.Path, len(rows), 10)
	page := &operatorsPage{
		Rows:  rows[start:end],
		Pager: pager,
	}
	if ctx.Query("new") == "1" {
		page.Dialog = admin.operatorForm(ctx, identity.OperatorDTO{}, pager.Current)
		return page, nil
	}

	if id := ctx.Query("edit"); id != "" {
		index := slices.IndexFunc(rows, func(row identity.OperatorDTO) bool {
			return row.ID == id && !row.SuperAdmin
		})
		if index >= 0 {
			page.Dialog = admin.operatorForm(ctx, rows[index], pager.Current)
		}
	}

	return page, nil
}

func (admin *admin) operatorForm(ctx *gin.Context, account identity.OperatorDTO, page int) *operatorDialog {
	account = admin.catalog.EffectiveGrants(account)
	dialog := &operatorDialog{
		ID:           account.ID,
		Username:     account.Username,
		Disabled:     account.Disabled,
		AllPages:     account.AllPages,
		AllEndpoints: account.AllEndpoints,
		Page:         page,
		CSRF:         csrfToken(sessionToken(ctx)),
	}
	for _, choice := range admin.catalog.Pages {
		dialog.Pages = append(dialog.Pages, operatorChoice{
			Key:     choice.Path,
			Label:   choice.Label,
			Checked: slices.Contains(account.Pages, choice.Path),
		})
	}

	for _, choice := range admin.catalog.Endpoints {
		dialog.Endpoints = append(dialog.Endpoints, operatorChoice{
			Key:     choice.Key,
			Label:   choice.Label,
			Checked: slices.Contains(account.Endpoints, choice.Key),
		})
	}

	return dialog
}

func (admin *admin) saveOperator(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, operatorsRoute, "") {
		return
	}

	request := ctx.Request.Context()
	actorID := currentOperator(ctx).ID
	id := ctx.Param("id")
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")
	disabled := ctx.PostForm("disabled") == "1"
	grants := identity.OperatorGrants{
		AllPages:     ctx.PostForm("all_pages") == "1",
		AllEndpoints: ctx.PostForm("all_endpoints") == "1",
		Pages:        append([]string{}, ctx.PostFormArray("pages")...),
		Endpoints:    append([]string{}, ctx.PostFormArray("endpoints")...),
	}
	pageNumber, _ := strconv.Atoi(ctx.Query("page"))
	pageNumber = max(pageNumber, 1)
	err := admin.catalog.Validate(grants)
	success := "运营账号已创建"
	if err == nil && id == "" {
		_, err = admin.services.CreateOperator(request, actorID, identity.CreateOperatorInput{
			Username:       username,
			Password:       password,
			OperatorGrants: grants,
		})
	}

	if err == nil && id != "" {
		_, err = admin.services.UpdateOperator(request, actorID, id, identity.UpdateOperatorInput{
			Password:       password,
			Disabled:       &disabled,
			OperatorGrants: grants,
		})
		success = "运营账号已更新"
	}

	status := http.StatusOK
	var dialog *operatorDialog
	if err != nil {
		original := identity.OperatorDTO{
			ID:           id,
			Username:     username,
			Disabled:     disabled,
			AllPages:     grants.AllPages,
			AllEndpoints: grants.AllEndpoints,
			Pages:        grants.Pages,
			Endpoints:    grants.Endpoints,
		}
		dialog = admin.operatorForm(ctx, original, pageNumber)
		status, dialog.Error = adminErrorMessage(ctx, err)
		if partialRequest(ctx) {
			ctx.Header("X-Admin-Form", "operator")
			admin.render(ctx, status, "operator-dialog", dialog)
			return
		}
	} else if !partialRequest(ctx) {
		redirect(ctx, operatorsRoute.Path+"?page="+strconv.Itoa(pageNumber))
		return
	}

	data := admin.workspace(ctx, operatorsRoute)
	data.Operators, err = admin.loadOperators(ctx)
	if err != nil {
		status, data.Error = adminErrorMessage(ctx, err)
	} else if dialog != nil {
		data.Operators.Dialog = dialog
	} else {
		data.Success = success
	}

	admin.renderWorkspace(ctx, status, data)
}
