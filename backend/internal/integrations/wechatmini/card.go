package wechatmini

import (
	"context"
	"errors"
	"net/url"

	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
)

func (c *PhoneClient) SendCard(ctx context.Context, openid string, card wxnotice.Card) (int, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return 0, wxnotice.ErrBeforeSend
	}

	var result struct {
		Code *int `json:"errcode"`
	}
	payload := struct {
		To string `json:"touser"`
		wxnotice.Card
	}{
		To:   openid,
		Card: card,
	}
	if err = c.post(ctx, "/cgi-bin/message/subscribe/send?access_token="+url.QueryEscape(token), payload, &result); err != nil || result.Code == nil {
		return 0, errors.New("未收到微信订阅消息回执")
	}

	return *result.Code, nil
}
