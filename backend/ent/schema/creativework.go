package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type CreativeWork struct{ ent.Schema }

func (CreativeWork) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}}
}

func (CreativeWork) Fields() []ent.Field {
	return []ent.Field{
		field.String("task_id"),

		field.Other("publish_fen", decimal.Decimal{}).SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.String("user_id"),

		field.String("business_date"),

		field.String("request_key").Unique(),

		field.Enum("status").GoType(enum.CreativeWorkStatus("")).Default(string(enum.CreativeDraft)),

		field.Int("revision").Default(0),

		field.String("copy_text").Default(""),

		field.JSON("files", []string{}).Optional(),

		field.String("reason").Default(""),

		field.String("reviewer_id").Default(""),
	}
}

func (CreativeWork) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id", "task_id").Unique(), index.Fields("user_id", "business_date")}
}

func (CreativeWork) Annotations() []schema.Annotation {
	return []schema.Annotation{mixin.EnumChecks((CreativeWork{}).Fields())}
}
