package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type UserPlatformProfile struct{ ent.Schema }

func (UserPlatformProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (UserPlatformProfile) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Unique().Immutable(),
		field.String("douyin").Default(""),
		field.String("xhs").Default(""),
	}
}

func (UserPlatformProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("platform_profile").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}
