package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type MarketingTag struct{ ent.Schema }

func (MarketingTag) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}, mixin.SoftDelete{}}
}

func (MarketingTag) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").NotEmpty().Immutable(),
		field.String("name").NotEmpty(),
	}
}

func (MarketingTag) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").
			Unique().
			StorageKey("marketing_tag_active_name").
			Annotations(entsql.IndexWhere("NOT deleted")),
		index.Fields("key").
			Unique().
			StorageKey("marketing_tag_active_key").
			Annotations(entsql.IndexWhere("NOT deleted")),
	}
}
