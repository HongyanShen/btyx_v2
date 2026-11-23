package agreement

import "testing"

func TestRichBodyProducesCompatiblePlainText(t *testing.T) {
	const input = "<h2>总则</h2><p>阅读 <strong>条款</strong> &amp; 确认</p><ol><li>甲</li><li>乙</li></ol><ul><li>丙</li></ul><p>&lt;标签&gt;<br>第二行</p>"
	body, rich, err := agreementBody(input)
	if err != nil || body != "总则\n阅读 条款 & 确认\n1. 甲\n2. 乙\n• 丙\n<标签>\n第二行" {
		t.Fatalf("unexpected plain text %q, err=%v", body, err)
	}

	if rich != "<h2>总则</h2><p>阅读 <strong>条款</strong> &amp; 确认</p><ol><li>甲</li><li>乙</li></ol><ul><li>丙</li></ul><p>&lt;标签&gt;<br/>第二行</p>" {
		t.Fatalf("unexpected rich body %q", rich)
	}
}

func TestRichBodyRequiresVisibleTextAndSupportedFormatting(t *testing.T) {
	for _, input := range []string{"<p><br></p>", "<p>&nbsp;</p>", "<p>\u200b</p>", "<img src='example'>", "<p class='other'>正文</p>"} {
		if _, _, err := agreementBody(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
