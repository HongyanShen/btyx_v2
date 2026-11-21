package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type Assignment struct{ ent.Schema }

func (Assignment) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Assignment) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),

		field.String("task_id"),

		field.String("version_id"),

		field.Other("price_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.JSON("platforms", []enum.Platform{}).
			Optional(),

		field.Enum("status").
			GoType(enum.AssignmentStatus("")).
			Default(string(enum.AssignmentInProgress)),

		field.Enum("source").
			GoType(enum.AssignmentSource("")),

		field.String("source_id").
			Default(""),

		field.String("business_date"),

		field.String("due_date"),

		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),

		field.String("quota_key").
			Optional().
			Nillable().
			Unique(),

		field.String("allocation_key").
			Optional().
			Nillable().
			Unique(),

		field.String("request_key").
			Unique(),

		field.JSON("steps", []enum.TaskStep{}).
			Optional(),

		field.String("actor_id"),
	}
}

func (Assignment) Indexes() []ent.Index {
	return []ent.Index{index.Fields("status", "expires_at")}
}

func (Assignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((Assignment{}).Fields()),
	}
}
