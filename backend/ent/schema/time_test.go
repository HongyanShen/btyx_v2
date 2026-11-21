package schema_test

import (
	"testing"

	"baituber.cn/yanxuan/v2/ent/schema"
	"baituber.cn/yanxuan/v2/ent/schema/mixin"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
)

func TestInstantsUseTimezoneAwareTimeFields(t *testing.T) {
	cases := []struct {
		name     string
		fields   []ent.Field
		nullable bool
	}{
		{
			name:   "created_at",
			fields: (mixin.Base{}).Fields(),
		},
		{
			name:   "updated_at",
			fields: (mixin.Mutable{}).Fields(),
		},
		{
			name:   "expires_at",
			fields: (schema.Session{}).Fields(),
		},
		{
			name:   "next_at",
			fields: (schema.Job{}).Fields(),
		},
		{
			name:     "approved_at",
			fields:   (schema.Submission{}).Fields(),
			nullable: true,
		},
		{
			name:     "upload_expires_at",
			fields:   (schema.FileObject{}).Fields(),
			nullable: true,
		},
		{
			name:     "verified_at",
			fields:   (schema.FileObject{}).Fields(),
			nullable: true,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for _, candidate := range test.fields {
				descriptor := candidate.Descriptor()
				if descriptor.Name != test.name {
					continue
				}

				if descriptor.Info.Type != field.TypeTime || descriptor.SchemaType[dialect.Postgres] != "timestamptz" {
					t.Errorf("%s must use time.Time / timestamptz, got %v / %q", test.name, descriptor.Info.Type, descriptor.SchemaType[dialect.Postgres])
				}

				if descriptor.Optional != test.nullable || descriptor.Nillable != test.nullable {
					t.Errorf("%s nullable = %v/%v, want %v", test.name, descriptor.Optional, descriptor.Nillable, test.nullable)
				}

				return
			}

			t.Fatalf("missing field %s", test.name)
		})
	}
}
