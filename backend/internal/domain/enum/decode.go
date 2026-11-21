package enum

import (
	"encoding/json"
	"fmt"
	"slices"
)

func valid(value string, allowed []string) bool {
	return slices.Contains(allowed, value)
}

func decode[T ~string](data []byte, destination *T, allowed []string) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	if !valid(value, allowed) {
		return fmt.Errorf("invalid enum value %q", value)
	}

	*destination = T(value)

	return nil
}
