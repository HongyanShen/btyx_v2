package enum

import (
	"encoding/json"
	"testing"
)

func TestPlatformRejectsUnknownJSONValues(t *testing.T) {
	for _, input := range []string{`"unknown"`, `""`, `null`, `12`} {
		var platform Platform
		if err := json.Unmarshal([]byte(input), &platform); err == nil {
			t.Errorf("accepted invalid platform: %s", input)
		}
	}
}

func TestEnumUsesStableTextRepresentation(t *testing.T) {
	encoded, err := json.Marshal(AssignmentPendingSettlement)
	if err != nil {
		t.Fatal(err)
	}

	if string(encoded) != `"pending_settlement"` {
		t.Fatalf("unexpected wire value: %s", encoded)
	}
}
