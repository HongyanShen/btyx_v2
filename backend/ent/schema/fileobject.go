package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type FileObject struct{ ent.Schema }

func (FileObject) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (FileObject) Fields() []ent.Field {
	return []ent.Field{
		field.String("owner_id"),

		field.String("name"),

		field.String("path").
			Default(""),

		field.Enum("media_type").
			GoType(enum.FileMediaType("")),

		field.Int64("size"),

		field.Enum("storage").
			GoType(enum.FileStorage("")).
			Default(string(enum.FileStorageLocal)),

		field.Enum("status").
			GoType(enum.FileStatus("")).
			Default(string(enum.FileReady)),

		field.String("bucket").
			Default(""),

		field.String("upload_key").
			Default(""),

		field.String("object_key").
			Default(""),

		field.String("version_id").
			Default(""),

		field.String("etag").
			Default(""),
		field.Bool("identity_private").
			Default(false),

		field.String("request_key").
			Optional().
			Nillable().
			Unique(),

		field.Time("upload_expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),

		field.Time("verified_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Optional().
			Nillable(),
	}
}

func (FileObject) Annotations() []schema.Annotation {
	return []schema.Annotation{mixin.EnumChecks((FileObject{}).Fields())}
}
