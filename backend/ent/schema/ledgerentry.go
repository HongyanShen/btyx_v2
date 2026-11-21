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

type LedgerEntry struct{ ent.Schema }

func (LedgerEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (LedgerEntry) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),

		field.String("business_key").
			Unique(),

		field.Enum("kind").
			GoType(enum.LedgerKind("")),

		field.Other("amount_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.String("reference_id"),

		field.String("original_id").
			Default(""),

		field.String("reason").
			Default(""),
	}
}

func (LedgerEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((LedgerEntry{}).Fields()),
	}
}
