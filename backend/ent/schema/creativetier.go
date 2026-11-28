package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type CreativeTier struct{ ent.Schema }

func (CreativeTier) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (CreativeTier) Fields() []ent.Field {
	return []ent.Field{
		field.String("task_id"),

		field.Int("likes").Positive(),

		field.Other("reward_fen", decimal.Decimal{}).SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),
	}
}

func (CreativeTier) Indexes() []ent.Index {
	return []ent.Index{index.Fields("task_id", "likes").Unique()}
}
