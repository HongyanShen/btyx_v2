package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Task struct{ ent.Schema }

func (Task) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}, mixin.SoftDelete{}, mixin.TaskNumber{}}
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

func (Task) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("display_number").
			Unique().
			Annotations(entsql.IndexWhere("NOT deleted")),
	}
}
