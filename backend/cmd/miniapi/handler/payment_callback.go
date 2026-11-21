package handler

import (
	"net/http"

	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func (handler *Handler) transferCallback(ctx *gin.Context) (httpx.EmptyResponse, error) {
	if handler.payments == nil {
		return httpx.EmptyResponse{}, &httpx.StatusError{Status: http.StatusServiceUnavailable}
	}

	bill, err := handler.payments.ParseNotification(ctx.Request.Context(), ctx.Request)
	if err != nil {
		return httpx.EmptyResponse{}, &httpx.StatusError{Status: http.StatusUnauthorized}
	}

	result, err := bill.TerminalResult()
	if err != nil {
		return httpx.EmptyResponse{}, &httpx.StatusError{Status: http.StatusBadRequest}
	}

	record, err := handler.services.Client.Withdrawal.Get(ctx.Request.Context(), bill.OutBillNo)
	if err != nil {
		return httpx.EmptyResponse{}, &httpx.StatusError{Status: http.StatusNotFound}
	}

	openID, err := handler.services.WechatOpenID(ctx.Request.Context(), record.UserID)
	if err != nil || openID != result.OpenID || record.AmountFen.String() != result.AmountFen {
		return httpx.EmptyResponse{}, &httpx.StatusError{Status: http.StatusBadRequest}
	}

	if err := handler.services.ApplyTransferResult(ctx.Request.Context(), record.ID, result.Status, result.ProviderID); err != nil {
		return httpx.EmptyResponse{}, &httpx.StatusError{Status: http.StatusInternalServerError}
	}

	return httpx.EmptyResponse{Status: http.StatusNoContent}, nil
}
