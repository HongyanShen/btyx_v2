package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// AuditHTTPResponse uses the server request ID as its ID. One HTTP operation may
// produce multiple immutable business events before its response is available.
type AuditHTTPResponse struct{ ent.Schema }

func (AuditHTTPResponse) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (AuditHTTPResponse) Fields() []ent.Field {
	return []ent.Field{
		field.Int("status").Range(100, 599).Immutable(),
		field.String("content_type").Immutable(),
		field.Bool("is_file").Immutable(),
		field.String("body_cipher").Default("").Sensitive().Immutable(),
	}
}
