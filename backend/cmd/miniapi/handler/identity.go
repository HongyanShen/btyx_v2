package handler

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type loginWechatInput struct {
	Code string `json:"code"`
}

type loginLocalInput struct {
	Device string `json:"device"`
}

type verifyPhoneInput struct {
	Code          string `json:"code"`
	UseForContact bool   `json:"use_for_contact"`
}

type recognizeIdentityInput struct {
	FileID string `json:"file_id"`
}

type agreementDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	BodyHTML string `json:"body_html,omitempty"`
}

type consentAgreementInput struct {
	AgreementID string `json:"agreement_id"`
}

func (handler *Handler) loginWechat(ctx *gin.Context, input loginWechatInput) (identity.LoginResult, error) {
	return handler.services.LoginWechat(ctx.Request.Context(), input.Code)
}

func (handler *Handler) loginLocal(ctx *gin.Context, input loginLocalInput) (identity.LoginResult, error) {
	return handler.services.DemoLogin(ctx.Request.Context(), input.Device)
}

func (handler *Handler) logout(ctx *gin.Context) (*httpx.NoData, error) {
	return nil, handler.services.LogoutUser(ctx.Request.Context(), httpapi.BearerToken(ctx))
}

func (handler *Handler) profile(ctx *gin.Context) (identity.UserDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")
	return handler.services.User(ctx.Request.Context(), httpapi.CurrentUser(ctx))
}

func (handler *Handler) updateProfile(ctx *gin.Context, input identity.UpdateProfileInput) (*httpx.NoData, error) {
	return nil, handler.services.UpdateProfile(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) verifyPhone(ctx *gin.Context, input verifyPhoneInput) (identity.VerifiedPhoneDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")
	return handler.services.VerifyPhone(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.Code, input.UseForContact)
}

func (handler *Handler) recognizeIdentity(ctx *gin.Context, input recognizeIdentityInput) (identity.RecognizedIdentity, error) {
	ctx.Header("Cache-Control", "private, no-store")
	return handler.services.RecognizeIdentity(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.FileID)
}

func (handler *Handler) verifyIdentity(ctx *gin.Context, input identity.VerifyIdentityInput) (*httpx.NoData, error) {
	return nil, handler.services.VerifyIdentity(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) applyEnrollment(ctx *gin.Context, input enrollment.ApplyInput) (*httpx.NoData, error) {
	return nil, handler.services.Apply(ctx.Request.Context(), httpapi.CurrentUser(ctx), input)
}

func (handler *Handler) currentAgreement(ctx *gin.Context) (agreementDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")
	record, err := handler.services.CurrentAgreement(ctx.Request.Context())
	if ent.IsNotFound(err) {
		return agreementDTO{}, app.Conflict("当前营销分发任务协议尚未发布")
	}

	if err != nil {
		return agreementDTO{}, err
	}

	return agreementDTO{
		ID:       record.ID,
		Title:    record.Title,
		Body:     record.Body,
		BodyHTML: record.BodyHTML,
	}, nil
}

func (handler *Handler) consentAgreement(ctx *gin.Context, input consentAgreementInput) (*httpx.NoData, error) {
	return nil, handler.services.ConsentVersion(ctx.Request.Context(), httpapi.CurrentUser(ctx), input.AgreementID)
}
