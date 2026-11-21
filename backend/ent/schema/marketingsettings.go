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

		field.Enum("expiry_mode").
			GoType(enum.MarketingExpiryMode("")).
			Default(string(enum.MarketingExpiryFixedDuration)),

		field.Int("expiry_minutes").Default(60),
	}
}

func (MarketingSettings) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((MarketingSettings{}).Fields()),
	}
}
