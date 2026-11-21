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

type SettlementLine struct{ ent.Schema }

func (SettlementLine) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (SettlementLine) Fields() []ent.Field {
	return []ent.Field{
		field.String("batch_id"),

		field.String("submission_id").
			Unique(),

		field.String("assignment_id"),

		field.String("user_id"),

		field.Other("amount_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Enum("platform").
			GoType(enum.Platform("")),
	}
}

func (SettlementLine) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((SettlementLine{}).Fields()),
	}
}
