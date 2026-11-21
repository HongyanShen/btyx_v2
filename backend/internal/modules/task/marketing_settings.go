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
	PriceFen      string                   `json:"price_fen"`
	ExpiryMode    enum.MarketingExpiryMode `json:"expiry_mode"`
	ExpiryMinutes int                      `json:"expiry_minutes"`
	Online        bool                     `json:"online"`
	PriceChanges  []MarketingPriceChange   `json:"price_changes"`
}

type MarketingSettingsInput struct {
	PriceFen      *string                   `json:"price_fen,omitempty"`
	ExpiryMode    *enum.MarketingExpiryMode `json:"expiry_mode,omitempty"`
	ExpiryMinutes *int                      `json:"expiry_minutes,omitempty"`
	Online        *bool                     `json:"online,omitempty"`
}

type MarketingPolicy struct {
	PriceFen        decimal.Decimal
	ExpiryMode      enum.MarketingExpiryMode
	ExpiryMinutes   int
	Online          bool
	priceOverridden bool
}

func LoadMarketingPolicy(ctx context.Context, client *ent.Client) (MarketingPolicy, error) {
	record, err := client.MarketingSettings.Query().
		Where(marketingsettings.ScopeEQ(marketingScope)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return MarketingPolicy{
			PriceFen:      decimal.RequireFromString("750"),
			ExpiryMode:    enum.MarketingExpiryFixedDuration,
			ExpiryMinutes: 60,
			Online:        true,
		}, nil
	}

	if err != nil {
		return MarketingPolicy{}, err
	}

	return MarketingPolicy{
		PriceFen:        record.PriceFen,
		ExpiryMode:      record.ExpiryMode,
		ExpiryMinutes:   record.ExpiryMinutes,
		Online:          record.Online,
		priceOverridden: record.PriceOverridden,
	}, nil
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
		PriceFen:      policy.PriceFen.String(),
		ExpiryMode:    policy.ExpiryMode,
		ExpiryMinutes: policy.ExpiryMinutes,
		Online:        policy.Online,
		PriceChanges:  changes,
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
	if input.PriceFen == nil && input.ExpiryMode == nil && input.ExpiryMinutes == nil && input.Online == nil {
		return MarketingSettingsDTO{}, app.Invalid("请填写要修改的营销设置")
	}

	price := decimal.RequireFromString("750")
	var changes []string
	if input.PriceFen != nil {
		validated, err := money.ParseFen(*input.PriceFen)
		if err != nil || !validated.IsPositive() || validated.GreaterThan(decimal.RequireFromString("50000")) {
			return MarketingSettingsDTO{}, app.Invalid("营销单价须大于 0 元且不超过 500 元")
		}

		price = validated
	}

	expiryMode := input.ExpiryMode
	if expiryMode != nil && !expiryMode.Valid() {
		return MarketingSettingsDTO{}, app.Invalid("失效规则无效")
	}

	if input.ExpiryMinutes != nil {
		minutes := *input.ExpiryMinutes
		if !validMarketingExpiryMinutes(minutes) {
			return MarketingSettingsDTO{}, app.Invalid("固定时长须在 0.5 至 24 小时之间，以半小时为单位")
		}

		if expiryMode == nil {
			fixed := enum.MarketingExpiryFixedDuration
			expiryMode = &fixed
		}

		if *expiryMode != enum.MarketingExpiryFixedDuration {
			return MarketingSettingsDTO{}, app.Invalid("当天截止无需设置固定时长")
		}

		changes = append(changes, fmt.Sprintf("失效时长 %d 分钟", minutes))
	}

	if input.Online != nil {
		visibility := "下架营销分发"
		if *input.Online {
			visibility = "上架营销分发"
		}

		changes = append(changes, visibility)
	}

	return app.Transaction(ctx, s.Runtime, func(tx *ent.Tx) (MarketingSettingsDTO, error) {
		err := tx.MarketingSettings.Create().
			SetScope(marketingScope).
			SetPriceFen(decimal.RequireFromString("750")).
			OnConflictColumns(marketingsettings.FieldScope).
			Ignore().
			Exec(ctx)
		if err != nil {
			return MarketingSettingsDTO{}, err
		}

		record, err := tx.MarketingSettings.Query().
			Where(marketingsettings.ScopeEQ(marketingScope)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return MarketingSettingsDTO{}, err
		}

		if expiryMode != nil && *expiryMode == enum.MarketingExpiryFixedDuration &&
			input.ExpiryMinutes == nil && !validMarketingExpiryMinutes(record.ExpiryMinutes) {
			return MarketingSettingsDTO{}, app.Invalid("请设置 0.5 至 24 小时的固定时长")
		}

		if expiryMode != nil && *expiryMode != record.ExpiryMode {
			label := "固定时长"
			if *expiryMode == enum.MarketingExpiryEndOfDay {
				label = "当天截止（北京时间 24:00）"
			}

			changes = append(changes, "失效规则改为"+label)
		}

		update := tx.MarketingSettings.UpdateOneID(record.ID).
			SetNillableOnline(input.Online).
			SetNillableExpiryMode(expiryMode).
			SetNillableExpiryMinutes(input.ExpiryMinutes)
		if input.PriceFen != nil {
			update.SetPriceFen(price).SetPriceOverridden(true)
			if !record.PriceFen.Equal(price) {
				change := fmt.Sprintf("每平台单价 %s → %s 分", record.PriceFen.String(), price.String())
				changes = append([]string{change}, changes...)
			}
		}

		if err := update.Exec(ctx); err != nil {
			return MarketingSettingsDTO{}, err
		}

		if len(changes) != 0 {
			if err := app.Audit(ctx, tx, actorID, enum.AuditMarketingSettings, marketingScope, strings.Join(changes, "，")); err != nil {
				return MarketingSettingsDTO{}, err
			}
		}

		policy, err := LoadMarketingPolicy(ctx, tx.Client())
		if err != nil {
			return MarketingSettingsDTO{}, err
		}

		return marketingSettingsDTO(ctx, tx.Client(), policy)
	})
}

func validMarketingExpiryMinutes(minutes int) bool {
	return minutes >= 30 && minutes <= 24*60 && minutes%30 == 0
}
