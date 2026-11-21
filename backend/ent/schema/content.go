package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Content struct{ ent.Schema }

func (Content) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}, mixin.SoftDelete{}}
}

func (Content) Fields() []ent.Field {
	return []ent.Field{
		field.String("title"),

		field.String("body"),

		field.String("project").
			Default(""),

		field.JSON("files", []string{}).
			Optional(),

		field.JSON("tags", []string{}).
			Optional(),

		field.String("creator_id"),
	}
}
