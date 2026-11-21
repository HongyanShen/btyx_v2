package mixin

import (
	"encoding/json"
	"fmt"
	"strings"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
)

// EnumChecks keeps PostgreSQL checks aligned with the same enum values used by
// Ent and the application, including typed JSON collections.
func EnumChecks(definitions []ent.Field) entsql.Annotation {
	checks := make(map[string]string)

	for _, definition := range definitions {
		descriptor := definition.Descriptor()
		if descriptor.Info.Type == field.TypeEnum {
			values := make([]string, 0, len(descriptor.Enums))
			for _, value := range descriptor.Enums {
				values = append(values, sqlLiteral(value.V))
			}

			checks[descriptor.Name+"_enum"] = fmt.Sprintf(
				`"%s" IN (%s)`,
				descriptor.Name,
				strings.Join(values, ", "),
			)
		}

		switch descriptor.Name {
		case "platforms":
			checks["platforms_enum"] = collectionCheck(descriptor.Name, (enum.Platform("")).Values())
		case "steps":
			checks["steps_enum"] = collectionCheck(descriptor.Name, (enum.TaskStep("")).Values())
		}
	}

	return entsql.Annotation{Checks: checks}
}

func collectionCheck(column string, values []string) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf(
		`jsonb_typeof("%[1]s") = 'array' AND "%[1]s" <@ %[2]s::jsonb`,
		column,
		sqlLiteral(string(encoded)),
	)
}

func sqlLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
