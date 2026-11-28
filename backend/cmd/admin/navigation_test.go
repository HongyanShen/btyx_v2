package main

import (
	"testing"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
)

func TestNavigationOnlyIncludesAvailableMenus(t *testing.T) {
	adminGroups := navigation(identity.OperatorDTO{SuperAdmin: true}, "/tasks")
	if len(adminGroups) != 3 {
		t.Fatalf("expected three functional groups, got %d", len(adminGroups))
	}

	for _, group := range adminGroups {
		for _, entry := range group.Entries {
			if entry.Path == "" {
				t.Fatalf("placeholder in %s: %s", group.Label, entry.Label)
			}
		}
	}

	account := identity.OperatorDTO{
		Pages:     []string{"/feedback"},
		Endpoints: []string{"GET /feedback"},
	}
	groups := navigation(account, "/feedback")
	if len(groups) != 1 || len(groups[0].Entries) != 1 || !groups[0].Entries[0].Selected {
		t.Fatal("limited accounts should see only their available entries")
	}
}
