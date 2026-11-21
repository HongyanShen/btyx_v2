package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type Feedback struct{ ent.Schema }

func (Feedback) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Feedback) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),

		field.String("body"),

		field.String("reply").
			Default(""),

		field.Enum("status").
			GoType(enum.FeedbackStatus("")).
			Default(string(enum.FeedbackOpen)),

		field.String("actor_id").
			Default(""),
	}
}

func (Feedback) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((Feedback{}).Fields()),
	}
}
