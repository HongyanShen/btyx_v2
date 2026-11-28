package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type CreativeTask struct{ ent.Schema }

func (CreativeTask) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}, mixin.Mutable{}, mixin.TaskNumber{}}
}

func (CreativeTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("display_number").Unique(),
	}
}

func (CreativeTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("creator_id"),

		field.String("title"),

		field.String("requirements"),

		field.Enum("media_type").GoType(enum.FileMediaType("")),

		field.String("copy_text"),

		field.String("topics"),

		field.JSON("files", []string{}).Optional(),

		field.JSON("platforms", []enum.Platform{}),

		field.Other("creation_fen", decimal.Decimal{}).SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Other("publish_fen", decimal.Decimal{}).SchemaType(map[string]string{dialect.Postgres: "numeric(24,0)"}),

		field.Bool("combined_likes"),

		field.Int("publish_days").Positive(),

		field.Time("starts_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("ends_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("reward_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Bool("online").Default(true),
	}
}
