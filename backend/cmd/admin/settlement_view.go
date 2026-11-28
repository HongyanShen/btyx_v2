package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"github.com/shopspring/decimal"
)

func newSettlementWeek(week settlement.WeekDTO, today string, users map[string]*identity.UserDTO, canCredit, canExport bool) settlementWeek {
	view := settlementWeek{
		WeekDTO: week,
		Date:    settlementShortDate(week.Through),
		Due:     week.Due,
		Label:   settlementWeekName(week, today),
	}
	qualified := settlementQualified(week.Lines)
	view.Amount = settlementYuan(settlementAmount(qualified))
	view.People = settlementPeople(qualified)
	view.Projects = strings.Join(settlementProjectTotals(qualified), " · ")
	pending := settlementPending(qualified)
	ready := settlementReady(pending)
	var next *time.Time
	for _, line := range pending {
		if !line.Due && line.EligibleAt != nil && (next == nil || line.EligibleAt.Before(*next)) {
			next = line.EligibleAt
		}
	}

	if next != nil {
		view.NextCredit = next.In(beijing).Format("15:04")
	}

	paid := slices.ContainsFunc(week.Lines, func(line settlement.WeekLineDTO) bool { return line.CreditedAt != nil })
	view.Export = paid && canExport
	if len(ready) > 0 {
		view.Badge, view.Tone = "待核对", "waiting"
		if canCredit {
			view.Credit = fmt.Sprintf("核对无误，确认入账（%d 人 · %s）", settlementPeople(ready), settlementYuan(settlementAmount(ready)))
		}
	} else if len(pending) == 0 && paid {
		view.Badge, view.Tone = "已入账", "success"
	} else if len(pending) > 0 || !view.Due {
		view.Badge, view.Tone = "还没到结算日", "neutral"
		if strings.HasPrefix(view.Label, "本周") {
			view.Badge = "本周进行中"
		}
	}

	groups := make(map[string][]settlement.WeekLineDTO)
	var order []string
	for _, line := range week.Lines {
		if _, exists := groups[line.UserID]; !exists {
			order = append(order, line.UserID)
		}

		groups[line.UserID] = append(groups[line.UserID], line)
	}

	var waiting []string
	for _, id := range order {
		group := newSettlementGroup(id, users[id], groups[id], week.Start)
		view.Groups = append(view.Groups, group)
		if group.Waiting > 0 {
			view.Waiting++
			waiting = append(waiting, fmt.Sprintf("%s %d 条", group.Name, group.Waiting))
		} else if slices.ContainsFunc(group.lines, func(line settlement.WeekLineDTO) bool { return line.HasScreenshot }) {
			view.Done++
		}
	}

	view.WaitingLabel = strings.Join(waiting, "、")
	return view
}

func newSettlementGroup(id string, owner *identity.UserDTO, lines []settlement.WeekLineDTO, start string) settlementGroup {
	qualified := settlementQualified(lines)
	group := settlementGroup{
		ID:          id,
		Owner:       owner,
		Name:        "分发员",
		Projects:    settlementProjectTotals(qualified),
		Qualified:   settlementTaskCount(qualified),
		Prices:      settlementPrices(qualified),
		Amount:      settlementYuan(settlementAmount(qualified)),
		NeedsCredit: len(settlementPending(qualified)) > 0,
		Ready:       len(settlementReady(qualified)) > 0,
		lines:       lines,
	}
	if owner != nil {
		group.Name = label(owner.Name, label(owner.Nickname, "分发员"))
	}

	wait := make(map[string]bool)
	late := make(map[string]bool)
	var last *settlement.WeekLineDTO
	for i := range lines {
		line := &lines[i]
		if settlementWaiting(*line) {
			wait[line.AssignmentID] = true
		}

		if line.TaskDate < start && line.AmountFen != "0" {
			late[line.AssignmentID] = true
		}

		if line.AmountFen != "0" && line.CreditedAt != nil && (last == nil || line.CreditedAt.After(*last.CreditedAt)) {
			last = line
		}

		if line.Visibility == enum.VisibilityUnavailable || line.Status == enum.SubmissionInvalid || line.Status == enum.SubmissionWaived || line.AmountFen == "0" && line.HasScreenshot {
			group.Checks = append(group.Checks, settlementCheck{
				Text:   settlementShortDate(line.PublishedOn) + "《" + line.Title + "》" + platformNames[line.Platform] + "：" + settlementCheckText(*line),
				Danger: line.Visibility == enum.VisibilityUnavailable,
			})
		}
	}

	group.Waiting, group.Late = len(wait), len(late)
	if len(group.Projects) == 0 {
		for _, line := range lines {
			if line.Project != "" && !slices.Contains(group.Projects, line.Project) {
				group.Projects = append(group.Projects, line.Project)
			}
		}
	}

	if len(group.Checks) == 0 {
		var checks []string
		for _, line := range qualified {
			text := settlementCheckText(line)
			if !slices.Contains(checks, text) {
				checks = append(checks, text)
			}
		}

		if len(checks) > 0 {
			group.Checks = []settlementCheck{{Text: strings.Join(checks, " · ")}}
		}
	}

	if last != nil {
		group.Posted = last.CreditedAt.In(beijing).Format("1/2 15:04")
		if last.CreditedBy != "" {
			group.Posted += " · " + last.CreditedBy
		}
	}

	return group
}

func newSettlementDetail(week settlementWeek, group settlementGroup) *settlementDetail {
	detail := &settlementDetail{
		Title:   group.Name + " · " + week.Label + " 结算明细",
		Amount:  group.Amount,
		Waiting: group.Amount == "¥0.00" && group.Waiting > 0,
	}
	if group.Owner != nil {
		detail.Account = group.Owner.Douyin
	}

	for _, line := range group.lines {
		amount, _ := decimal.NewFromString(line.AmountFen)
		detail.Rows = append(detail.Rows, settlementDetailRow{
			WeekLineDTO:  line,
			Date:         settlementShortDate(line.PublishedOn),
			PlatformName: platformNames[line.Platform],
			Amount:       settlementYuan(amount),
			Waiting:      settlementWaiting(line),
			Check: settlementCheck{
				Text:   settlementCheckText(line),
				Danger: line.Visibility == enum.VisibilityUnavailable,
			},
		})
	}

	return detail
}

func settlementShortDate(value string) string {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return "—"
	}

	return date.Format("1/2")
}

func settlementWeekName(week settlement.WeekDTO, today string) string {
	if week.Start == week.End {
		return settlementShortDate(week.Start)
	}

	name := settlementShortDate(week.Start) + "–" + settlementShortDate(week.End)
	if !week.Weekly {
		return name
	}

	if today >= week.Start && today <= week.End {
		return "本周 " + name
	}

	through, _ := time.Parse(time.DateOnly, week.Through)
	current, _ := time.Parse(time.DateOnly, today)
	if today >= week.Through && current.Before(through.AddDate(0, 0, 7)) {
		return "上周 " + name
	}

	return name
}

func settlementWaiting(line settlement.WeekLineDTO) bool {
	return !line.Creative && !line.HasScreenshot && line.Status != enum.SubmissionInvalid
}

func settlementCheckText(line settlement.WeekLineDTO) string {
	if line.Creative {
		return line.Project
	}

	if line.Status == enum.SubmissionInvalid && line.Visibility != enum.VisibilityUnavailable {
		return "↻ 已重新派发，这条不结算"
	}

	if line.Status == enum.SubmissionWaived {
		return "素材问题，照常结算"
	}

	switch line.Visibility {
	case enum.VisibilityVisible:
		return "✓ 游客看得到"
	case enum.VisibilityManualVerified:
		return "✓ 已人工确认可见"
	case enum.VisibilityUnavailable:
		return "✕ 游客看不到，不结算"
	default:
		return "待确认链接可见"
	}
}

func settlementQualifies(line settlement.WeekLineDTO) bool {
	return line.AmountFen != "0"
}

func settlementQualified(lines []settlement.WeekLineDTO) []settlement.WeekLineDTO {
	var result []settlement.WeekLineDTO
	for _, line := range lines {
		if settlementQualifies(line) {
			result = append(result, line)
		}
	}

	return result
}

func settlementPending(lines []settlement.WeekLineDTO) []settlement.WeekLineDTO {
	var result []settlement.WeekLineDTO
	for _, line := range lines {
		if settlementQualifies(line) && line.CreditedAt == nil {
			result = append(result, line)
		}
	}

	return result
}

func settlementReady(lines []settlement.WeekLineDTO) []settlement.WeekLineDTO {
	var result []settlement.WeekLineDTO
	for _, line := range lines {
		if line.Due && settlementQualifies(line) && line.CreditedAt == nil {
			result = append(result, line)
		}
	}

	return result
}

func settlementAmount(lines []settlement.WeekLineDTO) decimal.Decimal {
	amount := decimal.Zero
	for _, line := range lines {
		value, _ := decimal.NewFromString(line.AmountFen)
		amount = amount.Add(value)
	}

	return amount
}

func settlementPeople(lines []settlement.WeekLineDTO) int {
	people := make(map[string]bool)
	for _, line := range lines {
		people[line.UserID] = true
	}

	return len(people)
}

func settlementTaskCount(lines []settlement.WeekLineDTO) int {
	tasks := make(map[string]bool)
	for _, line := range lines {
		tasks[line.AssignmentID] = true
	}

	return len(tasks)
}

func settlementProjectTotals(lines []settlement.WeekLineDTO) []string {
	projects := make(map[string]decimal.Decimal)
	var order []string
	for _, line := range lines {
		project := label(line.Project, "未指定项目")
		if _, exists := projects[project]; !exists {
			order = append(order, project)
		}

		price, _ := decimal.NewFromString(line.AmountFen)
		projects[project] = projects[project].Add(price)
	}

	result := make([]string, 0, len(order))
	for _, project := range order {
		result = append(result, project+" "+settlementYuan(projects[project]))
	}

	return result
}

func settlementPrices(lines []settlement.WeekLineDTO) []string {
	prices := make(map[string]int)
	var result []string
	var order []string
	for _, line := range lines {
		if line.Creative {
			amount, _ := decimal.NewFromString(line.AmountFen)
			result = append(result, line.Project+" "+settlementYuan(amount))
			continue
		}

		if prices[line.AmountFen] == 0 {
			order = append(order, line.AmountFen)
		}

		prices[line.AmountFen]++
	}

	for _, fen := range order {
		price, _ := decimal.NewFromString(fen)
		result = append(result, fmt.Sprintf("%s × %d 个平台", settlementYuan(price), prices[fen]))
	}

	return result
}
