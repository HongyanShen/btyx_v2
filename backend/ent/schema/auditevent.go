package schema

import (
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AuditEvent struct{ ent.Schema }

func (AuditEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{mixin.Base{}}
}

func (AuditEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("actor_id"),

		field.Enum("action").
			GoType(enum.AuditAction("")),

		field.String("reference_id"),

		field.String("reason").
			Default(""),

		field.String("endpoint").Default("").Immutable(),
		field.String("request_id").MaxLen(27).Optional().Immutable(),

		field.Enum("platform").GoType(enum.AuditPlatform("")).Optional().Immutable(),

		field.String("parameters").Default("").Immutable(),

		field.String("submitted_phone").Default("").Immutable(),

		field.String("submitted_wechat_cipher").Default("").Sensitive().Immutable(),

		field.String("submitted_masked_name").Default("").Immutable(),

		field.String("submitted_masked_identity").Default("").Immutable(),

		field.String("submitted_front_file_id").Default("").Immutable(),

		field.String("submitted_back_file_id").Default("").Immutable(),
	}
}

func (AuditEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at", "id"),
		index.Fields("request_id").Annotations(entsql.IndexWhere("request_id IS NOT NULL")),
		index.Fields("endpoint", "created_at", "id"),
		index.Fields("submitted_front_file_id").
			Annotations(entsql.IndexWhere("submitted_front_file_id <> ''")),
		index.Fields("submitted_back_file_id").
			Annotations(entsql.IndexWhere("submitted_back_file_id <> ''")),
	}
}

func (AuditEvent) Annotations() []schema.Annotation {
	checks := mixin.EnumChecks((AuditEvent{}).Fields())
	// Replace the previous action check so existing databases accept failed requests.
	checks.Checks["audit_action_enum_v7"] = checks.Checks["action_enum"]
	delete(checks.Checks, "action_enum")

	return []schema.Annotation{
		checks,
	}
}
