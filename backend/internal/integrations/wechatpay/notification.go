package wechatpay

import (
	"context"
	"fmt"
	"net/http"
)

func (gateway *Gateway) ParseNotification(ctx context.Context, request *http.Request) (Bill, error) {
	var bill Bill
	if _, err := gateway.notifications.ParseNotifyRequest(ctx, request, &bill); err != nil {
		return bill, fmt.Errorf("微信通知验签或解密失败")
	}

	if bill.MerchantID != gateway.config.MerchantID || bill.OutBillNo == "" || bill.ProviderID == "" {
		return bill, fmt.Errorf("微信通知商户身份或单据信息不匹配")
	}

	return bill, nil
}
