package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/marketingsettings"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/money"
	"github.com/shopspring/decimal"
)

const marketingScope = "marketing"

type MarketingSettingsDTO struct {
	DailyLimit     int  `json:"daily_limit"`
	CreativeLimit  int  `json:"creative_limit"`
	CreativeOnline bool `json:"creative_online"`

	PriceFen      string                   `json:"price_fen"`
	ExpiryMode    enum.MarketingExpiryMode `json:"expiry_mode"`
	ExpiryMinutes int                      `json:"expiry_minutes"`
	Online        bool                     `json:"online"`
	PriceChanges  []MarketingPriceChange   `json:"price_changes"`
}

type MarketingSettingsInput struct {
	DailyLimit     *int  `json:"daily_limit,omitempty"`
	CreativeLimit  *int  `json:"creative_limit,omitempty"`
	CreativeOnline *bool `json:"creative_online,omitempty"`

	PriceFen      *string                   `json:"price_fen,omitempty"`
	ExpiryMode    *enum.MarketingExpiryMode `json:"expiry_mode,omitempty"`
	ExpiryMinutes *int                      `json:"expiry_minutes,omitempty"`
	Online        *bool                     `json:"online,omitempty"`
}

type MarketingPolicy struct {
	DailyLimit      int
	CreativeLimit   int
	CreativeOnline  bool
	PriceFen        decimal.Decimal
	ExpiryMode      enum.MarketingExpiryMode
	ExpiryMinutes   int
	Online          bool
	Schedule        MarketingSchedule
	priceOverridden bool
}

func LoadMarketingPolicy(ctx context.Context, client *ent.Client) (MarketingPolicy, error) {
	record, err := client.MarketingSettings.Query().
		Where(marketingsettings.ScopeEQ(marketingScope)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return MarketingPolicy{
			DailyLimit:     1,
			CreativeLimit:  1,
			CreativeOnline: true,
			PriceFen:       decimal.RequireFromString("750"),
			ExpiryMode:     enum.MarketingExpiryFixedDuration,
			ExpiryMinutes:  60,
			Online:         true,
			Schedule:       defaultMarketingSchedule(),
		}, nil
	}

	if err != nil {
		return MarketingPolicy{}, err
	}

	return MarketingPolicy{
		DailyLimit:      record.DailyLimit,
		CreativeLimit:   record.CreativeLimit,
		CreativeOnline:  record.CreativeOnline,
		PriceFen:        record.PriceFen,
		ExpiryMode:      record.ExpiryMode,
		ExpiryMinutes:   record.ExpiryMinutes,
		Online:          record.Online,
		Schedule:        marketingSchedule(record),
		priceOverridden: record.PriceOverridden,
	}, nil
}

func marketingSchedule(record *ent.MarketingSettings) MarketingSchedule {
	return MarketingSchedule{
		SettlementFrequency:     record.SettlementFrequency,
		SettlementWeekday:       record.SettlementWeekday,
		SettlementMonthDay:      record.SettlementMonthDay,
		SettlementClock:         record.SettlementClock,
		SettlementDelayMinutes:  record.SettlementDelayMinutes,
		PlaybackCutoffClock:     record.PlaybackCutoffClock,
		PlaybackCutoffMinutes:   record.PlaybackCutoffMinutes,
		PlaybackDeferralPeriods: record.PlaybackDeferralPeriods,
	}
}

func (policy MarketingPolicy) AllocationPrice(versionPrice decimal.Decimal) decimal.Decimal {
	if policy.priceOverridden {
		return policy.PriceFen
	}

	return versionPrice
}

func (policy MarketingPolicy) Deadline(start time.Time) time.Time {
	if policy.ExpiryMode == enum.MarketingExpiryEndOfDay {
		local := app.BeijingTime(start)
		year, month, day := local.Date()
		return time.Date(year, month, day+1, 0, 0, 0, 0, local.Location())
	}

	return start.Add(time.Duration(policy.ExpiryMinutes) * time.Minute)
}

func marketingSettingsDTO(ctx context.Context, client *ent.Client, policy MarketingPolicy) (MarketingSettingsDTO, error) {
	changes, err := marketingPriceChanges(ctx, client)
	if err != nil {
		return MarketingSettingsDTO{}, err
	}

	return MarketingSettingsDTO{
		DailyLimit:     policy.DailyLimit,
		CreativeLimit:  policy.CreativeLimit,
		CreativeOnline: policy.CreativeOnline,
		PriceFen:       policy.PriceFen.String(),
		ExpiryMode:     policy.ExpiryMode,
		ExpiryMinutes:  policy.ExpiryMinutes,
		Online:         policy.Online,
		PriceChanges:   changes,
	}, nil
}

func (s *Service) MarketingSettings(ctx context.Context) (MarketingSettingsDTO, error) {
	policy, err := LoadMarketingPolicy(ctx, s.Client)
	if err != nil {
		return MarketingSettingsDTO{}, err
	}

	return marketingSettingsDTO(ctx, s.Client, policy)
}

func (s *Service) UpdateMarketingSettings(ctx context.Context, actorID string, input MarketingSettingsInput) (MarketingSettingsDTO, error) {
	policy, err := s.updateMarketingPolicy(ctx, actorID, input, nil)
	if err != nil {
		return MarketingSettingsDTO{}, err
	}

	return marketingSettingsDTO(ctx, s.Client, policy)
}

func (s *Service) updateMarketingPolicy(ctx context.Context, actorID string, input MarketingSettingsInput, schedule *MarketingSchedule) (MarketingPolicy, error) {
	if input.PriceFen == nil && input.ExpiryMode == nil && input.ExpiryMinutes == nil && input.Online == nil && schedule == nil && input.DailyLimit == nil && input.CreativeLimit == nil && input.CreativeOnline == nil {
		return MarketingPolicy{}, app.Invalid("请填写要修改的营销设置")
	}

	if (input.DailyLimit != nil && *input.DailyLimit < 1) || (input.CreativeLimit != nil && *input.CreativeLimit < 1) {
		return MarketingPolicy{}, app.Invalid("每日领取名额须为正整数")
	}

	price := decimal.RequireFromString("750")
	var changes []string
	if input.PriceFen != nil {
		validated, err := money.ParseFen(*input.PriceFen)
		if err != nil || !validated.IsPositive() || validated.GreaterThan(decimal.RequireFromString("50000")) {
			return MarketingPolicy{}, app.Invalid("营销单价须大于 0 元且不超过 500 元")
		}

		price = validated
	}

	expiryMode := input.ExpiryMode
	if expiryMode != nil && !expiryMode.Valid() {
		return MarketingPolicy{}, app.Invalid("失效规则无效")
	}

	if input.ExpiryMinutes != nil {
		minutes := *input.ExpiryMinutes
		if !validMarketingExpiryMinutes(minutes) {
			return MarketingPolicy{}, app.Invalid("固定时长须在 0.5 至 24 小时之间，以半小时为单位")
		}

		if expiryMode == nil {
			fixed := enum.MarketingExpiryFixedDuration
			expiryMode = &fixed
		}

		if *expiryMode != enum.MarketingExpiryFixedDuration {
			return MarketingPolicy{}, app.Invalid("当天截止无需设置固定时长")
		}

	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (MarketingPolicy, error) {
		err := tx.MarketingSettings.Create().
			SetScope(marketingScope).
			SetPriceFen(decimal.RequireFromString("750")).
			OnConflictColumns(marketingsettings.FieldScope).
			Ignore().
			Exec(ctx)
		if err != nil {
			return MarketingPolicy{}, err
		}

		record, err := tx.MarketingSettings.Query().
			Where(marketingsettings.ScopeEQ(marketingScope)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return MarketingPolicy{}, err
		}

		if expiryMode != nil && *expiryMode == enum.MarketingExpiryFixedDuration &&
			input.ExpiryMinutes == nil && !validMarketingExpiryMinutes(record.ExpiryMinutes) {
			return MarketingPolicy{}, app.Invalid("请设置 0.5 至 24 小时的固定时长")
		}

		if input.ExpiryMinutes != nil && *input.ExpiryMinutes != record.ExpiryMinutes {
			changes = append(changes, fmt.Sprintf("失效时长 %d → %d 分钟", record.ExpiryMinutes, *input.ExpiryMinutes))
		}

		if expiryMode != nil && *expiryMode != record.ExpiryMode {
			label := "固定时长"
			if *expiryMode == enum.MarketingExpiryEndOfDay {
				label = "当天截止（北京时间 24:00）"
			}

			changes = append(changes, "失效规则改为"+label)
		}

		if input.Online != nil && *input.Online != record.Online {
			visibility := "下架营销分发"
			if *input.Online {
				visibility = "上架营销分发"
			}

			changes = append(changes, visibility)
		}

		if input.DailyLimit != nil && *input.DailyLimit != record.DailyLimit {
			changes = append(changes, fmt.Sprintf("分发每日名额 %d → %d", record.DailyLimit, *input.DailyLimit))
		}

		if input.CreativeLimit != nil && *input.CreativeLimit != record.CreativeLimit {
			changes = append(changes, fmt.Sprintf("创作每日名额 %d → %d", record.CreativeLimit, *input.CreativeLimit))
		}

		if input.CreativeOnline != nil && *input.CreativeOnline != record.CreativeOnline {
			changes = append(changes, fmt.Sprintf("创作任务开关 %t → %t", record.CreativeOnline, *input.CreativeOnline))
		}

		update := tx.MarketingSettings.UpdateOneID(record.ID).
			SetNillableDailyLimit(input.DailyLimit).
			SetNillableCreativeLimit(input.CreativeLimit).
			SetNillableCreativeOnline(input.CreativeOnline).
			SetNillableOnline(input.Online).
			SetNillableExpiryMode(expiryMode).
			SetNillableExpiryMinutes(input.ExpiryMinutes)
		if schedule != nil && *schedule != marketingSchedule(record) {
			update.SetSettlementFrequency(schedule.SettlementFrequency).
				SetSettlementWeekday(schedule.SettlementWeekday).
				SetSettlementMonthDay(schedule.SettlementMonthDay).
				SetSettlementClock(schedule.SettlementClock).
				SetSettlementDelayMinutes(schedule.SettlementDelayMinutes).
				SetPlaybackCutoffClock(schedule.PlaybackCutoffClock).
				SetPlaybackCutoffMinutes(schedule.PlaybackCutoffMinutes).
				SetPlaybackDeferralPeriods(schedule.PlaybackDeferralPeriods)
			changes = append(changes, "时间规则改为"+schedule.auditDetail())
		}

		if input.PriceFen != nil {
			update.SetPriceFen(price).SetPriceOverridden(true)
			if !record.PriceFen.Equal(price) {
				change := fmt.Sprintf("每平台单价 %s → %s 分", record.PriceFen.String(), price.String())
				changes = append([]string{change}, changes...)
			}
		}

		if err := update.Exec(ctx); err != nil {
			return MarketingPolicy{}, err
		}

		if len(changes) != 0 {
			if err := app.Audit(ctx, tx, actorID, enum.AuditMarketingSettings, marketingScope, strings.Join(changes, "，"),
				strings.Join(changes, "；")); err != nil {
				return MarketingPolicy{}, err
			}
		}

		return LoadMarketingPolicy(ctx, tx.Client())
	})
}

func validMarketingExpiryMinutes(minutes int) bool {
	return minutes >= 30 && minutes <= 24*60 && minutes%30 == 0
}
