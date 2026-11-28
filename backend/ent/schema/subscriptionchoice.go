package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SubscriptionChoice struct{ ent.Schema }

func (SubscriptionChoice) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (SubscriptionChoice) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Immutable(),
		field.String("attempt").Immutable(),
		field.String("scene").Immutable(),
		field.String("template_id").Immutable(),
		field.Enum("result").GoType(enum.SubscriptionResult("")).Immutable(),
		field.Time("observed_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.String("remembered").Default("").Immutable(),
		field.Bool("main_switch").Optional().Nillable().Immutable(),
		field.Int("error_code").Default(0).Immutable(),
		field.String("used_by").Default(""),
	}
}

func (SubscriptionChoice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "attempt", "template_id").Unique(),
		index.Fields("user_id", "template_id", "observed_at"),
	}
}
