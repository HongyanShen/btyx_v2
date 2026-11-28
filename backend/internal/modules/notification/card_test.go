package notification

import (
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"testing"
)

func TestReviewUsesApprovedTemplateFields(t *testing.T) {
	card := wxnotice.ReviewCard("创作作品", false, "素材不完整", "pages/work/index")
	if card.TemplateID != wxnotice.Review || card.Data["phrase1"].Value != "未通过" || card.Data["thing5"].Value != "素材不完整" {
		t.Fatal("审核结果模板映射错误")
	}
}
