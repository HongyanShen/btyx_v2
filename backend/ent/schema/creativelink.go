package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type CreativeLink struct{ ent.Schema }

func (CreativeLink) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (CreativeLink) Fields() []ent.Field {
	return []ent.Field{
		field.String("work_id"),

		field.Enum("platform").GoType(enum.Platform("")),

		field.String("url"),

		field.String("screenshot").Default(""),

		field.String("evidence_reason").Default(""),

		field.Int("likes").NonNegative().Optional().Nillable(),
	}
}

func (CreativeLink) Indexes() []ent.Index {
	return []ent.Index{index.Fields("work_id", "platform").Unique()}
}

func (CreativeLink) Annotations() []schema.Annotation {
	return []schema.Annotation{mixin.EnumChecks((CreativeLink{}).Fields())}
}
