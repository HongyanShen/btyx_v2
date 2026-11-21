package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Task struct{ ent.Schema }

func (Task) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}, mixin.SoftDelete{}}
}

func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.String("current_version_id").
			Default(""),

		field.String("creator_id"),

		field.Bool("online").
			Default(true),
	}
}
