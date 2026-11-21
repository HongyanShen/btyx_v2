package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type UserVerification struct{ ent.Schema }

func (UserVerification) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (UserVerification) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Unique().Immutable(),
		field.String("name_cipher").Default(""),
		field.String("identity_cipher").Default(""),
		field.String("verified_phone_cipher").Default(""),
		field.String("contact_phone_cipher").Default("").Sensitive(),
		field.String("wechat_cipher").Default(""),
		field.String("front_file_id").Default(""),
		field.String("back_file_id").Default(""),
		field.Time("verified_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),
	}
}

func (UserVerification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("verification").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}
