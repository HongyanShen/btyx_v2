package task

import (
	"context"
	"fmt"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
)

type MarketingSchedule struct {
	SettlementFrequency     enum.SettlementFrequency `json:"settlement_frequency"`
	SettlementWeekday       int                      `json:"settlement_weekday"`
	SettlementMonthDay      int                      `json:"settlement_month_day"`
	SettlementClock         string                   `json:"settlement_clock"`
	SettlementDelayMinutes  int                      `json:"settlement_delay_minutes"`
	PlaybackCutoffClock     string                   `json:"playback_cutoff_clock"`
	PlaybackCutoffMinutes   int                      `json:"playback_cutoff_minutes"`
	PlaybackDeferralPeriods int                      `json:"playback_deferral_periods"`
}

type DistributionSettings struct {
	ExpiryMode    enum.MarketingExpiryMode `json:"expiry_mode"`
	ExpiryMinutes int                      `json:"expiry_minutes"`
	Schedule      MarketingSchedule        `json:"schedule"`
}

func defaultMarketingSchedule() MarketingSchedule {
	return MarketingSchedule{
		SettlementFrequency:     enum.SettlementWeekly,
		SettlementWeekday:       1,
		SettlementMonthDay:      1,
		SettlementClock:         "00:00",
		SettlementDelayMinutes:  1440,
		PlaybackCutoffClock:     "12:00",
		PlaybackCutoffMinutes:   1440,
		PlaybackDeferralPeriods: 1,
	}
}

// Bound the date offsets and the relative duration before multiplying by a period count.
const maxDeferralPeriods = 1200
const maxScheduleMinutes = 52_560_000

func (schedule MarketingSchedule) validate() error {
	if !schedule.SettlementFrequency.Valid() {
		return app.Invalid("请选择结算方式")
	}

	if schedule.SettlementWeekday < 1 || schedule.SettlementWeekday > 7 {
		return app.Invalid("每周结算日期请选择星期一至星期日")
	}

	if schedule.SettlementMonthDay < 1 || schedule.SettlementMonthDay > 31 {
		return app.Invalid("每月结算日期须在 1 至 31 日之间")
	}

	for _, clock := range []string{schedule.SettlementClock, schedule.PlaybackCutoffClock} {
		parsed, err := time.Parse("15:04", clock)
		if err != nil || parsed.Format("15:04") != clock {
			return app.Invalid("请填写有效的北京时间，格式为 HH:mm")
		}
	}

	periods := schedule.PlaybackDeferralPeriods
	if periods < 1 || periods > maxDeferralPeriods {
		return app.Invalid(fmt.Sprintf("逾期顺延须填写 1 至 %d 期", maxDeferralPeriods))
	}

	if schedule.SettlementDelayMinutes < 1 || schedule.SettlementDelayMinutes > maxScheduleMinutes ||
		schedule.PlaybackCutoffMinutes < 1 || schedule.PlaybackCutoffMinutes > maxScheduleMinutes {
		return app.Invalid("请填写有效的结算等待和截图截止时长（整数分钟）")
	}

	if schedule.SettlementFrequency == enum.SettlementAfterDelay && schedule.SettlementDelayMinutes > maxScheduleMinutes/periods {
		return app.Invalid("结算等待时长与顺延期数的乘积过大")
	}

	return nil
}

func (s *Service) DistributionSettings(ctx context.Context) (DistributionSettings, error) {
	policy, err := LoadMarketingPolicy(ctx, s.Client)
	return policy.distributionSettings(), err
}

func (s *Service) UpdateDistributionSettings(ctx context.Context, actorID string, input DistributionSettings) (DistributionSettings, error) {
	if !input.ExpiryMode.Valid() {
		return DistributionSettings{}, app.Invalid("请选择首次提交发布链接的截止规则")
	}

	if err := input.Schedule.validate(); err != nil {
		return DistributionSettings{}, err
	}

	settings := MarketingSettingsInput{ExpiryMode: &input.ExpiryMode}
	if input.ExpiryMode == enum.MarketingExpiryFixedDuration {
		settings.ExpiryMinutes = &input.ExpiryMinutes
	}

	policy, err := s.updateMarketingPolicy(ctx, actorID, settings, &input.Schedule)
	return policy.distributionSettings(), err
}

func (policy MarketingPolicy) distributionSettings() DistributionSettings {
	return DistributionSettings{
		ExpiryMode:    policy.ExpiryMode,
		ExpiryMinutes: policy.ExpiryMinutes,
		Schedule:      policy.Schedule,
	}
}

func (schedule MarketingSchedule) auditDetail() string {
	playback := "结算日 " + schedule.PlaybackCutoffClock
	if schedule.SettlementFrequency == enum.SettlementAfterDelay {
		playback = fmt.Sprintf("分配后 %d 分钟", schedule.PlaybackCutoffMinutes)
	}

	return fmt.Sprintf("结算 %s，截图截止 %s，逾期顺延 %d 期", schedule.SettlementLabel(), playback, schedule.PlaybackDeferralPeriods)
}

func (schedule MarketingSchedule) SettlementLabel() string {
	switch schedule.SettlementFrequency {
	case enum.SettlementAfterDelay:
		return fmt.Sprintf("分配后 %d 分钟", schedule.SettlementDelayMinutes)
	case enum.SettlementDaily:
		return "每天 " + schedule.SettlementClock
	case enum.SettlementMonthly:
		return fmt.Sprintf("每月 %d 日 %s", schedule.SettlementMonthDay, schedule.SettlementClock)
	default:
		return fmt.Sprintf("每周%s %s", []string{"一", "二", "三", "四", "五", "六", "日"}[schedule.SettlementWeekday-1], schedule.SettlementClock)
	}
}

func (schedule MarketingSchedule) PlaybackDeadline(allocatedAt time.Time) time.Time {
	if schedule.SettlementFrequency == enum.SettlementAfterDelay {
		return app.BeijingTime(allocatedAt).Add(time.Duration(schedule.PlaybackCutoffMinutes) * time.Minute)
	}

	return scheduleTime(schedule.firstCalendarPeriod(allocatedAt), schedule.PlaybackCutoffClock)
}

func (schedule MarketingSchedule) EligibleAt(allocatedAt, screenshotAt time.Time) time.Time {
	late := !screenshotAt.IsZero() && !screenshotAt.Before(schedule.PlaybackDeadline(allocatedAt))
	if schedule.SettlementFrequency == enum.SettlementAfterDelay {
		wait := time.Duration(schedule.SettlementDelayMinutes) * time.Minute
		if late {
			return app.BeijingTime(screenshotAt).Add(wait * time.Duration(schedule.PlaybackDeferralPeriods))
		}

		return app.BeijingTime(allocatedAt).Add(wait)
	}

	period := schedule.firstCalendarPeriod(allocatedAt)
	if late {
		period = schedule.shiftPeriod(schedule.currentPeriod(screenshotAt), schedule.PlaybackDeferralPeriods)
	}

	return scheduleTime(period, schedule.SettlementClock)
}

func (schedule MarketingSchedule) CalendarPeriod(day time.Time) (start, end, opensAt time.Time, scheduled bool) {
	day = scheduleTime(day, "00:00")
	if schedule.SettlementFrequency == enum.SettlementAfterDelay || !schedule.currentPeriod(day).Equal(day) {
		return day, day, time.Time{}, false
	}

	opensAt = scheduleTime(day, schedule.SettlementClock)
	switch schedule.SettlementFrequency {
	case enum.SettlementDaily:
		start = day.AddDate(0, 0, -1)
		end = start
	case enum.SettlementMonthly:
		month := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
		start = month.AddDate(0, -1, 0)
		end = month.AddDate(0, 0, -1)
	default:
		monday := day.AddDate(0, 0, 1-schedule.SettlementWeekday)
		start = monday.AddDate(0, 0, -7)
		end = monday.AddDate(0, 0, -1)
	}

	return start, end, opensAt, true
}

func (schedule MarketingSchedule) firstCalendarPeriod(allocatedAt time.Time) time.Time {
	day := scheduleTime(allocatedAt, "00:00")
	switch schedule.SettlementFrequency {
	case enum.SettlementDaily:
		return day.AddDate(0, 0, 1)
	case enum.SettlementMonthly:
		return schedule.monthPeriod(day, 1)
	default:
		monday := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
		return monday.AddDate(0, 0, 6+schedule.SettlementWeekday)
	}
}

func (schedule MarketingSchedule) currentPeriod(at time.Time) time.Time {
	day := scheduleTime(at, "00:00")
	switch schedule.SettlementFrequency {
	case enum.SettlementDaily:
		return day
	case enum.SettlementMonthly:
		period := schedule.monthPeriod(day, 0)
		if period.After(day) {
			return schedule.monthPeriod(day, -1)
		}

		return period
	default:
		weekday := (int(day.Weekday())+6)%7 + 1
		return day.AddDate(0, 0, -((weekday - schedule.SettlementWeekday + 7) % 7))
	}
}

func (schedule MarketingSchedule) shiftPeriod(day time.Time, periods int) time.Time {
	switch schedule.SettlementFrequency {
	case enum.SettlementDaily:
		return day.AddDate(0, 0, periods)
	case enum.SettlementMonthly:
		return schedule.monthPeriod(day, periods)
	default:
		return day.AddDate(0, 0, 7*periods)
	}
}

func (schedule MarketingSchedule) monthPeriod(day time.Time, offset int) time.Time {
	month := time.Date(day.Year(), day.Month()+time.Month(offset), 1, 0, 0, 0, 0, day.Location())
	last := month.AddDate(0, 1, -1).Day()
	return month.AddDate(0, 0, min(schedule.SettlementMonthDay, last)-1)
}

func scheduleTime(day time.Time, clock string) time.Time {
	day = app.BeijingTime(day)
	value, _ := time.Parse("15:04", clock)
	return time.Date(day.Year(), day.Month(), day.Day(), value.Hour(), value.Minute(), 0, 0, day.Location())
}
