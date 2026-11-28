package wxnotice

import (
	"errors"
	"strings"
	"time"
	"unicode"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

var ErrBeforeSend = errors.New("尚未向微信发送订阅消息")

const (
	EnrollmentResult  = "lRpXa_GcSSdTfsMcNX1vl4fC9jCsvAiQSr_fjp66nls"
	Work              = "m7bP5ii__LKnQj4LZEnuV-ws68uJtV4YjWxqxC7aYVY"
	Status            = "XmoEA-LpkJM9vsUgPOSk1S6nbmqnTcM9gKBxZpkId58"
	WithdrawalFailed  = "ldjJdaBKIBQ-K7UwB1OosRUeFKcxJVx9KpXQFoUhA9c"
	WithdrawalSuccess = "9Qmhjyn6viQGoClFe4idKnufkOT5zrJPfzLV2EwC2Rk"
	Settlement        = "pLLqPxTGF6GwUliYDNIuApr6Xrj5qYvKLFR09_MYN6Q"
	Review            = "4Wtw2GKO6twy8zaUyNRhB6jl-pxvRAMJCARmwQXPjMI"
	Todo              = "KnXG2Q8yGUbnBsHGlpWml1_kacR2QpWNSpRUwSUKbpc"
	NewTask           = "RYd5-QWO-HO5Ur2UsI2qI1GkjA_7LrLj4DKk1eTLLhI"
)

var Scenes = map[string][]string{
	"tasks":       {NewTask, Status, Todo},
	"enrollment":  {EnrollmentResult, NewTask},
	"claim":       {NewTask, Todo, Work},
	"links":       {Status, Work, Settlement},
	"screenshots": {Settlement},
	"creative":    {Review, Status, Settlement},
	"withdrawal":  {WithdrawalFailed, WithdrawalSuccess},
}

type Value struct {
	Value string `json:"value"`
}
type Card struct {
	TemplateID string           `json:"template_id"`
	Page       string           `json:"page"`
	Data       map[string]Value `json:"data"`
}

func text(value string) Value {
	value = strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
	runes := []rune(value)
	if len(runes) > 20 {
		value = string(runes[:20])
	}

	if value == "" {
		value = "无"
	}

	return Value{Value: value}
}

func Clock(at time.Time) string {
	return at.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02 15:04")
}

func Platform(p enum.Platform) string {
	if p == enum.PlatformDouyin {
		return "抖音"
	}

	return "小红书"
}

func Detail(id string) string {
	return "subpackages/task/detail/index?id=" + id
}

func Assignment(title, description, remark, id string, due time.Time) Card {
	return Card{NewTask, Detail(id), map[string]Value{
		"thing1": text(title),
		"thing2": text(description),
		"time3":  text(Clock(due)),
		"thing7": text(remark),
	}}
}

func TaskStatus(title, status, remark, page string) Card {
	return Card{Status, page, map[string]Value{
		"thing2":  text(title),
		"thing3":  text(status),
		"thing11": text(remark),
	}}
}

func LinkReminder(title, id string, due time.Time) Card {
	return Card{Todo, Detail(id), map[string]Value{
		"thing1":  text("提交发布链接"),
		"thing2":  text(title),
		"time3":   text(Clock(due)),
		"thing11": text("请在截止时间前提交"),
	}}
}

func Playback(title string, due time.Time) Card {
	return Card{Work, "subpackages/task/playback/index", map[string]Value{
		"thing2":  text(title),
		"thing4":  text("请补交播放数据截图"),
		"thing10": text("截止 " + Clock(due)),
	}}
}

func ReviewCard(title string, approved bool, reason, page string) Card {
	result := "未通过"
	if approved {
		result = "已通过"
		reason = "无"
	}

	return Card{Review, page, map[string]Value{
		"phrase1": text(result),
		"thing2":  text(title),
		"thing5":  text(reason),
	}}
}

func EnrollmentCard(approved bool, reason string) Card {
	result := "未通过"
	page := "subpackages/member/enrollment/index"
	if approved {
		result = "已通过"
		reason = "可前往查看今日任务"
		page = "subpackages/task/today/index"
	}

	return Card{EnrollmentResult, page, map[string]Value{
		"phrase1": {Value: result},
		"thing7":  text("营销分发员报名"),
		"thing5":  text(reason),
	}}
}

func Credit(amount string) Card {
	return Card{Settlement, "pages/wallet/index", map[string]Value{
		"thing1":   text("营销奖励结算"),
		"thing3":   text("已计入可提现余额"),
		"thing5":   text("营销任务结算"),
		"amount6":  {Value: amount + "元"},
		"phrase13": text("已入账"),
	}}
}

func Transfer(amount string, applied time.Time, success bool, reason string) Card {
	page := "subpackages/member/withdrawal-records/index"
	if success {
		return Card{WithdrawalSuccess, page, map[string]Value{
			"amount1":  {Value: amount + "元"},
			"amount14": {Value: amount + "元"},
			"thing4":   text("营销奖励提现"),
			"phrase3":  text("微信支付"),
		}}
	}

	return Card{WithdrawalFailed, page, map[string]Value{
		"amount1": {Value: amount + "元"},
		"time2":   text(Clock(applied)),
		"thing3":  text(reason),
		"thing4":  text("冻结金额已退回可提现余额"),
	}}
}
