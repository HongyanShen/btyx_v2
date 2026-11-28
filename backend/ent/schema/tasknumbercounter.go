package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type TaskNumberCounter struct{ ent.Schema }

func (TaskNumberCounter) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (TaskNumberCounter) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().Immutable().Unique(),
		field.Int64("last_number").NonNegative(),
	}
}
