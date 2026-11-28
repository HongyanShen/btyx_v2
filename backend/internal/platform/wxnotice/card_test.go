package wxnotice

import (
	"strings"
	"testing"
	"time"
)

func TestCardFieldLimits(t *testing.T) {
	exact := strings.Repeat("分", 19) + "𠮷"
	for _, input := range []string{exact, exact + "超出", "\n" + exact + "\t多余"} {
		card := EnrollmentCard(false, input)
		if card.Data["thing5"].Value != exact || card.FieldError() != "" {
			t.Fatal("超长备注须保留前二十个完整字符")
		}
	}

	for _, card := range []Card{
		EnrollmentCard(true, ""), Assignment(exact, exact, exact, "id", time.Now()),
		TaskStatus(exact, exact, exact, "page"), LinkReminder(exact, "id", time.Now()),
		Playback(exact, time.Now()), ReviewCard(exact, false, exact, "page"),
		Credit("99999999.99"), Transfer("0.01", time.Now(), true, ""),
		Transfer("123.45", time.Now(), false, exact),
	} {
		if field := card.FieldError(); field != "" {
			t.Fatalf("有效卡片被拒绝: %s", field)
		}
	}

	for field, value := range map[string]string{
		"phrase1": "审核结果已通过",
		"phrase2": "OK",
		"thing2":  exact + "长",
		"amount1": "12345678901.23元",
		"time3":   "无效日期",
	} {
		card := Card{Data: map[string]Value{field: {Value: value}}}
		if card.FieldError() != field {
			t.Fatalf("应标出不符合微信格式的字段 %s", field)
		}
	}
}
