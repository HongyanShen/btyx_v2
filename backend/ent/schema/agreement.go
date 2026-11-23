package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Agreement struct{ ent.Schema }

func (Agreement) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (Agreement) Fields() []ent.Field {
	return []ent.Field{
		field.String("title"),

		field.String("body"),

		field.Text("body_html").
			Optional(),

		field.String("creator_id"),

		field.Bool("active").
			Default(true),
	}
}
