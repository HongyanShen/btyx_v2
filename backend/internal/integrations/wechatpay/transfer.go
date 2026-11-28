package wechatpay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"baituber.cn/yanxuan/v2/internal/platform/transfer"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

type transferBody struct {
	AppID        string               `json:"appid"`
	OutBillNo    string               `json:"out_bill_no"`
	SceneID      string               `json:"transfer_scene_id"`
	OpenID       string               `json:"openid"`
	Amount       int64                `json:"transfer_amount"`
	Remark       string               `json:"transfer_remark"`
	NotifyURL    string               `json:"notify_url"`
	SceneReports []config.SceneReport `json:"transfer_scene_report_infos"`
}

type Bill struct {
	OutBillNo   string                   `json:"out_bill_no"`
	ProviderID  string                   `json:"transfer_bill_no"`
	State       enum.WechatTransferState `json:"state"`
	MerchantID  string                   `json:"mch_id"`
	AppID       string                   `json:"appid"`
	OpenID      string                   `json:"openid"`
	Amount      json.Number              `json:"transfer_amount"`
	PackageInfo string                   `json:"package_info"`
}

func (gateway *Gateway) Submit(ctx context.Context, request transfer.Request) (transfer.Result, error) {
	if request.OpenID == "" || strings.HasPrefix(request.OpenID, "local:") {
		logging.Warn(ctx).Msg("wechat transfer requires a real recipient identity")
		return transfer.Result{}, fmt.Errorf("禁止向本地演示身份发起真实转账")
	}

	amount, err := money.ParseFen(request.AmountFen)
	if err != nil || !amount.IsPositive() {
		return transfer.Result{}, fmt.Errorf("转账金额无效")
	}

	// The provider contract requires an integer JSON amount. Our own DTOs retain
	// string fen; conversion here is exact and never passes through a float.
	result, err := gateway.client.Post(ctx, transferEndpoint, transferBody{
		AppID:        gateway.appID,
		OutBillNo:    request.ID,
		SceneID:      gateway.config.SceneID,
		OpenID:       request.OpenID,
		Amount:       amount.IntPart(),
		Remark:       "白兔严选营销分发奖励",
		NotifyURL:    gateway.config.NotifyURL,
		SceneReports: gateway.config.SceneReports,
	})
	if err != nil {
		logSubmitError(ctx, err)
		return transfer.Result{}, fmt.Errorf("微信转账请求未确认，必须按原商户单号查单")
	}

	defer result.Response.Body.Close()
	var bill Bill
	if err := core.UnMarshalResponse(result.Response, &bill); err != nil {
		return transfer.Result{}, err
	}

	if bill.OutBillNo != request.ID {
		return transfer.Result{}, fmt.Errorf("微信商户单号与请求不一致")
	}

	return bill.result(request.AmountFen, request.OpenID)
}

func logSubmitError(ctx context.Context, err error) {
	event := logging.Error(ctx, err)
	var apiError *core.APIError
	if errors.As(err, &apiError) {
		code := apiError.Code
		switch code {
		case "PARAM_ERROR", "INVALID_REQUEST", "NO_AUTH", "SIGN_ERROR", "SYSTEM_ERROR", "NOT_ENOUGH", "FREQUENCY_LIMIT_EXCEED", "RATELIMIT_EXCEEDED", "FREQUENCY_LIMIT", "ALREADY_EXISTS":
		default:
			code = "UNKNOWN"
		}

		event.Int("wechat_http_status", apiError.StatusCode).
			Str("wechat_error_code", code).
			Str("wechat_rejection", submitRejectionCategory(apiError.Message))
	}

	event.Msg("wechat transfer submit failed")
}

func submitRejectionCategory(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "ip") && (strings.Contains(message, "不允许") || strings.Contains(message, "白名单")):
		return "ip_not_allowed"
	case strings.Contains(message, "报备"):
		return "scene_reporting"
	case strings.Contains(message, "场景"):
		return "transfer_scene"
	case strings.Contains(lower, "openid"):
		return "recipient_identity"
	case strings.Contains(lower, "appid"):
		return "app_identity"
	case strings.Contains(message, "余额"):
		return "balance"
	case strings.Contains(message, "金额"), strings.Contains(message, "限额"):
		return "amount_limit"
	default:
		return "unspecified"
	}
}

func (gateway *Gateway) Query(ctx context.Context, id string) (transfer.Result, error) {
	result, err := gateway.client.Get(ctx, transferEndpoint+"/out-bill-no/"+url.PathEscape(id))
	if err != nil {
		var apiError *core.APIError
		if errors.As(err, &apiError) && apiError.StatusCode == 404 && apiError.Code == "NOT_FOUND" {
			return transfer.Result{}, transfer.ErrNotFound
		}

		return transfer.Result{}, fmt.Errorf("微信查单未获得确定结果，继续保留冻结资金")
	}

	defer result.Response.Body.Close()
	var bill Bill
	if err := core.UnMarshalResponse(result.Response, &bill); err != nil {
		return transfer.Result{}, err
	}

	if bill.OutBillNo != id || bill.AppID != gateway.appID || bill.MerchantID != gateway.config.MerchantID {
		return transfer.Result{}, fmt.Errorf("微信查单的商户单号或 AppID 不一致")
	}

	return bill.result(bill.Amount.String(), bill.OpenID)
}

func (bill Bill) TerminalResult() (transfer.Result, error) {
	if bill.State != enum.WechatTransferSuccess && bill.State != enum.WechatTransferFailed && bill.State != enum.WechatTransferCancelled {
		return transfer.Result{}, fmt.Errorf("通知不是转账终态")
	}

	return bill.result(bill.Amount.String(), bill.OpenID)
}

func (bill Bill) result(amount, openID string) (transfer.Result, error) {
	if bill.ProviderID == "" || openID == "" {
		return transfer.Result{}, fmt.Errorf("微信转账响应缺少单号或收款身份")
	}

	if _, err := money.ParseFen(amount); err != nil {
		return transfer.Result{}, fmt.Errorf("微信转账响应金额无效")
	}

	status, err := mapState(bill.State)
	if err != nil {
		return transfer.Result{}, err
	}

	return transfer.Result{
		Status:      status,
		ProviderID:  bill.ProviderID,
		PackageInfo: bill.PackageInfo,
		AmountFen:   amount,
		OpenID:      openID,
	}, nil
}

func mapState(state enum.WechatTransferState) (enum.WithdrawalStatus, error) {
	switch state {
	case enum.WechatTransferSuccess:
		return enum.WithdrawalSuccess, nil
	case enum.WechatTransferFailed:
		return enum.WithdrawalFailed, nil
	case enum.WechatTransferCancelled:
		return enum.WithdrawalCancelled, nil
	case enum.WechatTransferWaitConfirm:
		return enum.WithdrawalWaitingConfirmation, nil
	case enum.WechatTransferAccepted, enum.WechatTransferProcessing, enum.WechatTransferTransferring, enum.WechatTransferCancelling:
		return enum.WithdrawalSubmitting, nil
	default:
		return enum.WithdrawalUnknown, fmt.Errorf("未知的微信转账状态，必须继续核对")
	}
}
