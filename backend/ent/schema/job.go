package schema

import (
	"time"

	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Job struct{ ent.Schema }

func (Job) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("kind").
			GoType(enum.JobKind("")),

		field.String("reference_id"),

		field.String("request_id").
			Default("").
			MaxLen(27).
			Immutable(),

		field.Enum("status").
			GoType(enum.JobStatus("")).
			Default(string(enum.JobQueued)),

		field.Int64("attempts").
			Default(0),

		field.Time("next_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Default(time.Now),

		field.String("last_error").
			Default(""),
	}
}

func (Job) Indexes() []ent.Index {
	return []ent.Index{index.Fields("kind", "reference_id").
		Unique()}
}

func (Job) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((Job{}).Fields()),
	}
}
