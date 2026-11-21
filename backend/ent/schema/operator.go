package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Operator struct{ ent.Schema }

func (Operator) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Operator) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").
			NotEmpty().
			MaxLen(64).
			Unique().
			Immutable(),

		field.String("password_hash").
			NotEmpty().
			Sensitive(),

		field.Bool("super_admin").
			Default(false).
			Immutable(),

		field.Bool("disabled").
			Default(false),

		field.JSON("pages", []string{}).
			Default([]string{}),

		field.JSON("endpoints", []string{}).
			Default([]string{}),
	}
}

func (Operator) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("super_admin").
			Unique().
			Annotations(entsql.IndexWhere("super_admin")),
	}
}

func (Operator) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "operators",
			Checks: map[string]string{
				"operator_super_admin_enabled": "NOT super_admin OR NOT disabled",
			},
		},
	}
}
