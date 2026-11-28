package main

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/gin-gonic/gin"
)

type operatorProfilePage struct {
	PasswordError string
}

func (admin *admin) profileWorkspace(ctx *gin.Context) workspaceData {
	data := admin.workspace(ctx, workspacePage{
		Path:  "/profile",
		Label: "个人信息",
	})
	data.Profile = &operatorProfilePage{}
	return data
}

func (admin *admin) profilePage(ctx *gin.Context) {
	admin.renderWorkspace(ctx, http.StatusOK, admin.profileWorkspace(ctx))
}

func (admin *admin) changeProfilePassword(ctx *gin.Context) {
	if !admin.allowForm(ctx) {
		return
	}

	password := ctx.PostForm("new_password")
	var err error
	if password != ctx.PostForm("confirm_password") {
		err = app.Invalid("两次输入的新密码不一致")
	} else {
		err = admin.services.ChangeOperatorPassword(ctx.Request.Context(), currentOperator(ctx).ID, ctx.PostForm("current_password"), password)
	}

	if err != nil {
		data := admin.profileWorkspace(ctx)
		status, message := adminErrorMessage(ctx, err)
		data.Profile.PasswordError = message
		admin.renderWorkspace(ctx, status, data)
		return
	}

	admin.setSession(ctx, "", -1)
	redirect(ctx, "/login?password_changed=1")
}
