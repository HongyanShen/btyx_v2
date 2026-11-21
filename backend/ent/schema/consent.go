package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Consent struct{ ent.Schema }

func (Consent) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (Consent) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),
		field.String("agreement_id"),
	}
}

func (Consent) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "agreement_id").
		Unique()}
}
