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
		index.Fields("submitted_front_file_id").
			Annotations(entsql.IndexWhere("submitted_front_file_id <> ''")),
		index.Fields("submitted_back_file_id").
			Annotations(entsql.IndexWhere("submitted_back_file_id <> ''")),
	}
}

func (AuditEvent) Annotations() []schema.Annotation {
	checks := mixin.EnumChecks((AuditEvent{}).Fields())
	// Replace the v5 check on databases that predate marketing settings and expiry audits.
	checks.Checks["audit_action_enum_v6"] = checks.Checks["action_enum"]
	delete(checks.Checks, "action_enum")

	return []schema.Annotation{
		checks,
	}
}
