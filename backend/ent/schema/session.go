package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type Session struct{ ent.Schema }

func (Session) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").
			Unique(),

		field.String("user_id"),

		field.Enum("role").
			GoType(enum.Role("")),

		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Session) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((Session{}).Fields()),
	}
}
