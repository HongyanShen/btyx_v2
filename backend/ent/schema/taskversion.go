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

type TaskVersion struct{ ent.Schema }

func (TaskVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (TaskVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("task_id"),

		field.Int64("number"),

		field.String("title"),

		field.String("body"),

		field.String("project").
			Default(""),

		field.String("content_id").
			Default(""),

		field.JSON("files", []string{}).
			Optional(),

		field.JSON("tags", []string{}).
			Optional(),

		field.JSON("platforms", []enum.Platform{}).
			Optional(),

		field.Other("price_fen", decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.String("deadline").
			Default(""),

		field.String("creator_id"),
	}
}

func (TaskVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("task_id", "number").
		Unique()}
}

func (TaskVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		mixin.EnumChecks((TaskVersion{}).Fields()),
	}
}
