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

type Withdrawal struct{ ent.Schema }

func (Withdrawal) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Withdrawal) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),

		field.String("request_key").
			Unique(),

		field.Other("amount_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Enum("status").
			GoType(enum.WithdrawalStatus("")).
			Default(string(enum.WithdrawalQueued)),

		field.String("provider_id").
			Default(""),

		field.String("reason").
			Default(""),

		field.String("package_info").
			Default(""),
	}
}

func (Withdrawal) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((Withdrawal{}).Fields()),
	}
}
