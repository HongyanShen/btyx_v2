package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserEnrollment struct{ ent.Schema }

func (UserEnrollment) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (UserEnrollment) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Immutable(),
		field.Bool("is_current").Default(true),
		field.String("applicant_nickname").MaxRuneLen(32).Default("").Immutable(),
		field.String("douyin").Optional().Nillable(),
		field.String("xhs").Optional().Nillable(),
		field.Enum("status").
			GoType(enum.EnrollmentStatus("")).
			Default(string(enum.EnrollmentNone)),
		field.String("contact_phone_cipher").Default(""),
		field.String("name_cipher").Default(""),
		field.String("identity_cipher").Default(""),
		field.String("front_file_id").Default(""),
		field.String("back_file_id").Default(""),
		field.Time("identity_checked_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),
		field.String("reason").Default(""),
		field.JSON("tags", []string{}).Optional(),
		field.Time("submitted_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),
		field.Bool("recent_post").Optional().Nillable(),
		field.Bool("new_account").Optional().Nillable(),
		field.Enum("age_group").
			GoType(enum.EnrollmentAge("")).
			Optional().
			Nillable(),
	}
}

func (UserEnrollment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("user_id").Unique().
			StorageKey("user_enrollment_current_user").
			Annotations(entsql.IndexWhere("is_current")),
	}
}

func (UserEnrollment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("enrollment").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (UserEnrollment) Annotations() []schema.Annotation {
	return []schema.Annotation{mixin.EnumChecks((UserEnrollment{}).Fields())}
}
