package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"github.com/shopspring/decimal"
)

type SettlementBatch struct{ ent.Schema }

func (SettlementBatch) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (SettlementBatch) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_key").
			Unique(),

		field.String("through"),

		field.String("creator_id"),

		field.Other("amount_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Int64("count"),
	}
}
