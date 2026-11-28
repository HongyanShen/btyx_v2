package business

import (
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/creative"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"testing"
	"time"
)

func TestCreativeScreenshotRewardAndCredit(t *testing.T) {
	s := testService(t)
	ctx := t.Context()
	now := time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	login, err := s.DemoLogin(ctx, "creative-flow")
	must(err)
	uid := login.UserID
	must(s.PrepareTestUser(ctx, uid))
	newFile := func() string {
		t.Helper()
		row, err := s.Client.FileObject.Create().
			SetOwnerID(uid).
			SetName("test.png").
			SetMediaType(enum.FilePNG).
			SetSize(10).
			Save(ctx)
		must(err)
		return row.ID
	}
	in := creative.TaskInput{
		Title:        "双平台创作",
		Requirements: "按主题发布",
		MediaType:    enum.FilePNG,
		CreationFen:  "4000",
		PublishDays:  1,
		StartsAt:     now.Add(-time.Hour),
		EndsAt:       now.Add(time.Hour),
		RewardAt:     now.Add(24 * time.Hour),
		Platforms:    []enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu},
		Tiers: []creative.Tier{{
			Likes:     3000,
			RewardFen: "26600",
		}},
	}
	id, err := s.Creative.Create(ctx, "operator", in)
	must(err)
	work, err := s.Creative.Claim(ctx, uid, id, "creative-claim-1126")
	must(err)
	off := false
	_, err = s.UpdateMarketingSettings(ctx, "operator", task.MarketingSettingsInput{CreativeOnline: &off})
	must(err)
	cat, err := s.Creative.Catalog(ctx, uid)
	must(err)
	if cat.Online || len(cat.Tasks) != 0 || len(cat.Mine) != 1 {
		t.Fatal("关闭后应仅保留已领取任务")
	}

	if code := cat.Mine[0].Task.TaskCode; len(code) != 7 || code[:3] != "RW-" {
		t.Fatalf("unexpected creative task code %q", code)
	}

	draft := creative.SubmitInput{
		Files:    []string{newFile()},
		CopyText: "创作文案",
	}
	must(s.Creative.Submit(ctx, uid, work, draft))
	must(s.Creative.Review(ctx, "operator", work, creative.ReviewInput{
		Revision: 1,
		Reason:   "补充话题",
	}))
	draft.Revision = 1
	must(s.Creative.Submit(ctx, uid, work, draft))
	must(s.Creative.Review(ctx, "operator", work, creative.ReviewInput{
		Revision: 2,
		Approve:  true,
	}))
	links := creative.LinksInput{Links: []creative.Link{{
		Platform: enum.PlatformDouyin,
		URL:      "https://v.douyin.com/sample/",
	}, {
		Platform: enum.PlatformXiaohongshu,
		URL:      "https://xhslink.cn/o/sample",
	}}}
	must(s.Creative.Links(ctx, uid, work, links))
	must(s.Creative.Links(ctx, uid, work, links))
	now = now.Add(25 * time.Hour)
	first := 6800
	second := 1200
	links.Links[0].Likes = &first
	links.Links[1].Likes = &second
	for i := range links.Links {
		links.Links[i].Screenshot = newFile()
		must(s.Creative.Evidence(ctx, uid, work, links.Links[i]))
	}

	returned := links.Links[0]
	returned.Reason = "请显示完整点赞数和账号"
	must(s.Creative.ReturnEvidence(ctx, "operator", work, returned))
	if s.Creative.Award(ctx, "operator", work, links) == nil {
		t.Fatal("退回截图后仍能核定奖励")
	}

	old := links.Links[0].Screenshot
	links.Links[0].Screenshot = newFile()
	must(s.Creative.Evidence(ctx, uid, work, links.Links[0]))
	stale := links.Links[0]
	stale.Screenshot = old
	if s.Creative.Award(ctx, "operator", work, creative.LinksInput{Links: []creative.Link{stale, links.Links[1]}}) == nil {
		t.Fatal("使用被替换的截图发奖")
	}

	must(s.Creative.Award(ctx, "operator", work, links))
	must(s.Creative.Award(ctx, "operator", work, links))
	balance, err := s.Wallet(ctx, uid)
	must(err)
	if balance.PendingFen != "32100" || balance.AvailableFen != "0" {
		t.Fatalf("入账前资金错误 %+v", balance)
	}

	now = now.AddDate(0, 0, 7)
	report, err := s.Month(ctx, "2026-11", settlement.MonthAccess{
		Pending: true,
		History: true,
	})
	must(err)
	for _, wk := range report.Weeks {
		if wk.ReviewToken != "" {
			input := settlement.SettlementInput{
				Key:         "credit-" + wk.Through,
				Through:     wk.Through,
				ReviewToken: wk.ReviewToken,
			}
			_, err = s.Settle(ctx, "operator", input)
			must(err)
			_, err = s.Settle(ctx, "operator", input)
			must(err)
		}
	}

	balance, err = s.Wallet(ctx, uid)
	must(err)
	if balance.AvailableFen != "32100" || balance.PendingFen != "0" {
		t.Fatalf("入账后资金错误 %+v", balance)
	}
}
