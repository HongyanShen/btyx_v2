package mixin

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/segmentio/ksuid"
)

type Base struct{ mixin.Schema }

func (Base) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			DefaultFunc(func() string {
				return ksuid.New().
					String()
			}).
			Immutable().
			MaxLen(27),

		field.Time("created_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Default(time.Now).
			Immutable(),
	}
}

type Mutable struct{ mixin.Schema }

func (Mutable) Fields() []ent.Field {
	return []ent.Field{
		field.Time("updated_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
