package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/platform/wxnotice"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Notification struct{ ent.Schema }

func (Notification) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id"),
		field.JSON("card", wxnotice.Card{}).Optional(),
		field.Enum("delivery_state").GoType(enum.DeliveryState("")).Default(string(enum.DeliveryStation)),
		field.String("delivery_reason").Default(""),
		field.Int("wechat_code").Default(0),
		field.String("choice_id").Default(""),

		field.String("title"),

		field.String("body"),

		field.String("reference_id").
			Default(""),

		field.Bool("read").
			Default(false),
	}
}
