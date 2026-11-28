package task

import (
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestMarketingScheduleUsesConfiguredBeijingCutoff(t *testing.T) {
	schedule := defaultMarketingSchedule()
	schedule.SettlementWeekday = 3
	schedule.SettlementClock = "14:30"
	schedule.PlaybackCutoffClock = "11:15"
	allocated := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 11, 25, 3, 15, 0, 0, time.UTC)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"before cutoff", cutoff.Add(-time.Second), "2026-11-25"},
		{"at cutoff", cutoff, "2026-12-02"},
		{"late by multiple cycles", cutoff.AddDate(0, 0, 14).Add(-time.Second), "2026-12-16"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := schedule.EligibleAt(allocated, test.at).Format(time.DateOnly); got != test.want {
				t.Fatalf("eligible date = %s, want %s", got, test.want)
			}
		})
	}

	open := time.Date(2026, 11, 25, 6, 30, 0, 0, time.UTC)
	if got := schedule.EligibleAt(allocated, cutoff.Add(-time.Second)); !got.Equal(open) {
		t.Fatalf("opening time = %v, want %v in Beijing", got, open)
	}
}

func TestMarketingScheduleDailyMonthlyAndAllocationDelay(t *testing.T) {
	schedule := defaultMarketingSchedule()
	schedule.SettlementClock = "15:20"
	schedule.PlaybackCutoffClock = "14:00"
	schedule.PlaybackDeferralPeriods = 2
	allocated := time.Date(2027, 1, 31, 1, 0, 0, 0, time.UTC)

	schedule.SettlementFrequency = enum.SettlementDaily
	if got := schedule.EligibleAt(allocated, allocated).Format(time.RFC3339); got != "2027-02-01T15:20:00+08:00" {
		t.Fatalf("daily settlement at %s", got)
	}

	schedule.SettlementFrequency = enum.SettlementMonthly
	schedule.SettlementMonthDay = 31
	if got := schedule.EligibleAt(allocated, allocated).Format(time.RFC3339); got != "2027-02-28T15:20:00+08:00" {
		t.Fatalf("short month settlement at %s", got)
	}

	late := time.Date(2027, 2, 28, 6, 0, 0, 0, time.UTC)
	if got := schedule.EligibleAt(allocated, late).Format(time.RFC3339); got != "2027-04-30T15:20:00+08:00" {
		t.Fatalf("monthly deferral at %s", got)
	}

	schedule.SettlementFrequency = enum.SettlementAfterDelay
	schedule.SettlementDelayMinutes = 90
	schedule.PlaybackCutoffMinutes = 75
	if got := schedule.EligibleAt(allocated, allocated.Add(time.Hour)); !got.Equal(allocated.Add(90 * time.Minute)) {
		t.Fatalf("allocation delay at %s", got)
	}

	submitted := allocated.Add(75 * time.Minute)
	if got := schedule.EligibleAt(allocated, submitted); !got.Equal(submitted.Add(3 * time.Hour)) {
		t.Fatalf("late allocation delay at %s", got)
	}
}
