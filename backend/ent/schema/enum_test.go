package schema

import (
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

func TestStateAndDiscriminatorFieldsUseEnums(t *testing.T) {
	entities := []struct {
		name   string
		fields []ent.Field
		keys   []string
	}{
		{
			name:   "UserEnrollment",
			fields: (UserEnrollment{}).Fields(),
			keys:   []string{"status"},
		},
		{
			name:   "Session",
			fields: (Session{}).Fields(),
			keys:   []string{"role"},
		},
		{
			name:   "Assignment",
			fields: (Assignment{}).Fields(),
			keys:   []string{"status", "source"},
		},
		{
			name:   "Submission",
			fields: (Submission{}).Fields(),
			keys:   []string{"platform", "status", "visibility"},
		},
		{
			name:   "Withdrawal",
			fields: (Withdrawal{}).Fields(),
			keys:   []string{"status"},
		},
		{
			name:   "LedgerEntry",
			fields: (LedgerEntry{}).Fields(),
			keys:   []string{"kind"},
		},
		{
			name:   "Job",
			fields: (Job{}).Fields(),
			keys:   []string{"kind", "status"},
		},
		{
			name:   "Feedback",
			fields: (Feedback{}).Fields(),
			keys:   []string{"status"},
		},
		{
			name:   "AuditEvent",
			fields: (AuditEvent{}).Fields(),
			keys:   []string{"action"},
		},
		{
			name:   "FileObject",
			fields: (FileObject{}).Fields(),
			keys:   []string{"storage", "status", "media_type"},
		},
	}

	for _, entity := range entities {
		for _, key := range entity.keys {
			found := false
			for _, definition := range entity.fields {
				descriptor := definition.Descriptor()
				if descriptor.Name != key {
					continue
				}

				found = true
				if descriptor.Info.Type != field.TypeEnum {
					t.Errorf("%s.%s is not an enum", entity.name, key)
				}
			}

			if !found {
				t.Errorf("missing %s.%s", entity.name, key)
			}
		}
	}
}
