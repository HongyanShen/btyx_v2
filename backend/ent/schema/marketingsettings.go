package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/shopspring/decimal"
)

type MarketingSettings struct{ ent.Schema }

func (MarketingSettings) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (MarketingSettings) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope").Unique().Immutable(),

		field.Other("price_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Bool("price_overridden").Default(false),

		field.Bool("online").Default(true),

		field.Int("daily_limit").Positive().Default(1),

		field.Int("creative_limit").Positive().Default(1),

		field.Bool("creative_online").Default(true),

		field.Enum("expiry_mode").
			GoType(enum.MarketingExpiryMode("")).
			Default(string(enum.MarketingExpiryFixedDuration)),

		field.Int("expiry_minutes").Default(60),

		field.Enum("settlement_frequency").
			GoType(enum.SettlementFrequency("")).
			Default(string(enum.SettlementWeekly)),

		field.Int("settlement_weekday").Default(1),

		field.Int("settlement_month_day").Default(1),

		field.String("settlement_clock").Default("00:00"),

		field.Int("settlement_delay_minutes").Default(24 * 60),

		field.String("playback_cutoff_clock").Default("12:00"),

		field.Int("playback_cutoff_minutes").Default(24 * 60),

		field.Int("playback_deferral_periods").Default(1),
	}
}

func (MarketingSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((MarketingSettings{}).Fields()),
	}
}
