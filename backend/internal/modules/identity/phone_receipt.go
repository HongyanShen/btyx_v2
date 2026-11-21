package identity

import (
	"encoding/json"
	"time"

	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type contactPhoneReceipt struct {
	UserID    string
	Number    string
	ExpiresAt time.Time
}

func (s *Service) issueContactPhoneReceipt(userID, number string) (string, error) {
	data, err := json.Marshal(contactPhoneReceipt{
		UserID:    userID,
		Number:    number,
		ExpiresAt: s.Now().Add(30 * time.Minute),
	})
	if err != nil {
		return "", err
	}

	return s.Encrypt(string(data))
}

func (s *Service) contactPhoneFromReceipt(userID, value string) (string, error) {
	var receipt contactPhoneReceipt
	if err := json.Unmarshal([]byte(s.Decrypt(value)), &receipt); err != nil ||
		receipt.UserID != userID || !receipt.ExpiresAt.After(s.Now()) ||
		!verifiedPhonePattern.MatchString(receipt.Number) {
		return "", app.Invalid("微信手机号授权无效或已过期，请重新获取")
	}

	return receipt.Number, nil
}
