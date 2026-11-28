package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// Both task types keep their originally allocated display number.
type TaskNumber struct{ mixin.Schema }

func (TaskNumber) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("display_number").Positive().Immutable(),
	}
}
