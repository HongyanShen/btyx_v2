package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct{ ent.Schema }

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}, mixin.SoftDelete{}}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("nickname").
			Default("").
			MaxRuneLen(32),

		field.String("avatar_file_id").
			Default("").
			MaxLen(27),

		field.Bool("blocked").
			Default(false),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("wechat_identity", WechatIdentity.Type).
			Unique(),
		edge.To("verification", UserVerification.Type).
			Unique(),
		edge.To("platform_profile", UserPlatformProfile.Type).
			Unique(),
		edge.To("enrollment", UserEnrollment.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nickname").
			Annotations(entsql.IndexWhere("NOT deleted")),
	}
}
