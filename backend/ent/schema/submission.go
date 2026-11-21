package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Submission struct{ ent.Schema }

func (Submission) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Submission) Fields() []ent.Field {
	return []ent.Field{
		field.String("assignment_id"),

		field.Enum("platform").
			GoType(enum.Platform("")),

		field.String("url"),

		field.String("work_key"),

		field.String("screenshot").
			Default(""),

		field.Enum("status").
			GoType(enum.SubmissionStatus("")).
			Default(string(enum.SubmissionPending)),

		field.String("reason").
			Default(""),

		field.String("reviewer_id").
			Default(""),

		field.Int64("revision").
			Default(1),

		field.Time("approved_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),

		field.String("settlement_id").
			Default(""),

		field.String("eligible_date").
			Default(""),

		field.Enum("visibility").
			GoType(enum.Visibility("")).
			Default(string(enum.VisibilityUnchecked)),
	}
}

func (Submission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("assignment_id", "platform").
			Unique(),
		index.Fields("platform", "work_key").
			Unique(),
	}
}

func (Submission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((Submission{}).Fields()),
	}
}
