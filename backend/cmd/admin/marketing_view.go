package main

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/task"
)

var bodyTagLine = regexp.MustCompile(`(?i)^TAG[：:](.*)$`)
var bodyMentionLine = regexp.MustCompile(`^@账号[：:](.*)$`)
var bodyMinimumLine = regexp.MustCompile(`^话题至少带满 ([0-9]+) 个（含必带 TAG）\s*$`)

func splitMarketingBody(body string) marketingCopy {
	result := marketingCopy{}
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if parts := bodyTagLine.FindStringSubmatch(line); len(parts) != 0 {
			result.Tags = strings.TrimSpace(parts[1])
		} else if parts := bodyMentionLine.FindStringSubmatch(line); len(parts) != 0 {
			result.Mention = strings.TrimSpace(parts[1])
		} else if parts := bodyMinimumLine.FindStringSubmatch(line); len(parts) != 0 {
			result.Minimum = parts[1]
			result.MinText = line
		} else {
			lines = append(lines, line)
		}
	}

	result.Text = strings.TrimSpace(strings.Join(lines, "\n"))
	return result
}

func marketingRowView(version task.VersionDTO, users map[string]identity.UserDTO, tags map[string]string, today time.Time, canAssign bool) marketingRow {
	date := version.CreatedAt
	if version.UploadedAt != nil {
		date = *version.UploadedAt
	}

	row := marketingRow{
		VersionDTO: version,
		Artist:     label(version.Project, "未分项目"),
		UploadDate: date.In(beijing).Format(time.DateOnly),
		Media:      marketingMedia(version),
		State:      "排队中",
		Copy:       splitMarketingBody(version.Body),
	}
	var platforms []string
	for _, platform := range version.Platforms {
		platforms = append(platforms, platformNames[platform])
	}

	row.PlatformText = strings.Join(platforms, " + ")
	if len(platforms) == 1 {
		row.PlatformText = "只发" + row.PlatformText
	}

	for _, key := range version.Tags {
		row.TagLabels = append(row.TagLabels, label(tags[key], key))
	}

	if allocation := version.Allocation; allocation != nil {
		row.State = "已领取"
		if allocation.HasLink {
			row.State = "已发布"
		}

		person := users[allocation.UserID]
		row.Recipient = marketingMemberName(person)
		row.RecipientNote = person.Douyin
		if allocation.Source == enum.SourceAssigned {
			row.RecipientNote = label(person.Douyin+" · 工作人员指派", "工作人员指派")
			if person.Douyin == "" {
				row.RecipientNote = "工作人员指派"
			}
		}

		if !allocation.HasLink && allocation.ExpiresAt != nil {
			row.Expiry = allocation.ExpiresAt.In(beijing).Format("1/2 15:04")
		}

		row.Steps, row.StepName = marketingSteps(allocation)
	} else if !version.Online {
		row.State = "已下架"
	} else if version.Deadline != "" && version.Deadline < today.Format(time.DateOnly) {
		row.State = "已截止"
	}

	if last := version.LastExpiry; last != nil {
		source := "领取"
		if last.Source == enum.SourceAssigned {
			source = "被指派"
		} else if last.Source == enum.SourceReissued {
			source = "接到重派"
		}

		row.LastExpired = marketingMemberName(users[last.UserID]) + source + "后超时未交链接，任务已失效。"
		if last.ExpiredAt != nil {
			row.LastExpired += " " + last.ExpiredAt.In(beijing).Format("1/2 15:04")
		}
	}

	row.CanAssign = canAssign && row.State == "排队中"
	return row
}

func marketingMedia(version task.VersionDTO) string {
	video := slices.Contains(version.MediaTypes, enum.FileMP4)
	image := slices.ContainsFunc(version.MediaTypes, func(media enum.FileMediaType) bool { return media != enum.FileMP4 })
	switch {
	case video && image:
		return "图文、视频"
	case video:
		return "视频"
	case image:
		return "图文"
	case len(version.Files) > 0:
		return "附件"
	default:
		return "文案"
	}
}

func marketingSteps(allocation *task.PoolAllocationDTO) ([]marketingStep, string) {
	names := []string{"拿内容", "发布", "交链接", "审核", "结算"}
	completed := []bool{
		slices.Contains(allocation.Steps, enum.StepContent),
		slices.Contains(allocation.Steps, enum.StepPublish),
		allocation.LinksDone,
		allocation.Reviewed,
		allocation.Settled,
	}
	steps := make([]marketingStep, len(names))
	current := ""
	for i, name := range names {
		step := marketingStep{
			Label: name,
			Done:  completed[i],
		}
		if step.Done {
			step.Class = "is-complete"
		} else if current == "" {
			current = name
			step.Class = "is-current"
		}

		steps[i] = step
	}

	return steps, label(current, "已结算")
}

func marketingMemberName(user identity.UserDTO) string {
	return label(user.Name, label(user.Nickname, label(user.Douyin, "分发员")))
}

func marketingMatches(keys []string, users []identity.UserDTO, tags []enrollment.MarketingTagDTO) []string {
	type match struct {
		user identity.UserDTO
		keys []string
	}
	names := make(map[string]string, len(tags))
	for _, tag := range tags {
		names[tag.Key] = tag.Name
	}

	var matches []match
	for _, user := range users {
		var titles []string
		for _, key := range keys {
			if slices.Contains(user.Tags, key) {
				titles = append(titles, label(names[key], key))
			}
		}

		if len(titles) > 0 {
			matches = append(matches, match{
				user: user,
				keys: titles,
			})
		}
	}

	slices.SortStableFunc(matches, func(left, right match) int { return len(right.keys) - len(left.keys) })
	var result []string
	for _, item := range matches {
		name := marketingMemberName(item.user)
		if item.user.Douyin != "" {
			name += "（" + item.user.Douyin + "）"
		}

		result = append(result, name+" · 标签对上 "+strings.Join(item.keys, "、"))
	}

	return result
}
