package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type WechatIdentity struct{ ent.Schema }

func (WechatIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (WechatIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Unique().Immutable(),
		field.String("openid").Unique().Immutable(),
	}
}

func (WechatIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("wechat_identity").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}
