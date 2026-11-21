package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// MarketingTagDeletion records the original tag ID and its permanently retired key.
type MarketingTagDeletion struct{ ent.Schema }

func (MarketingTagDeletion) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (MarketingTagDeletion) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").NotEmpty().Unique().Immutable(),
	}
}
