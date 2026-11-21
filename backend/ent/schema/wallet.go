package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"github.com/shopspring/decimal"
)

type Wallet struct{ ent.Schema }

func (Wallet) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Wallet) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").
			Unique(),

		field.Other("available_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Other("frozen_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Other("withdrawn_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),
	}
}
