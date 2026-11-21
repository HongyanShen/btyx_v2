package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Notification struct{ ent.Schema }

func (Notification) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),

		field.String("title"),

		field.String("body"),

		field.String("reference_id").
			Default(""),

		field.Bool("read").
			Default(false),
	}
}
