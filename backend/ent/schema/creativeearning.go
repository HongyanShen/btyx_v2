package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type CreativeEarning struct{ ent.Schema }

func (CreativeEarning) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (CreativeEarning) Fields() []ent.Field {
	return []ent.Field{
		field.String("work_id"),

		field.String("user_id"),

		field.Bool("likes_reward"),

		field.Other("amount_fen", decimal.Decimal{}).SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Time("eligible_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.String("batch_id").Default(""),
	}
}

func (CreativeEarning) Indexes() []ent.Index {
	return []ent.Index{index.Fields("work_id", "likes_reward").Unique()}
}
