package operations

import (
	"slices"
	"testing"

	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
)

func TestFullGrantsFollowCatalogAdditions(t *testing.T) {
	catalog := newOperatorCatalog(nil, []httpx.Route{{
		Method: "GET",
		Path:   "/tasks",
	}})
	stored := identity.OperatorDTO{
		Pages:     []string{"/tasks"},
		Endpoints: []string{"GET /tasks"},
	}
	catalog.Pages = append(catalog.Pages, identity.OperatorPage{Path: "/new-page"})
	catalog.Endpoints = append(catalog.Endpoints, identity.OperatorEndpoint{Key: "GET /new-endpoint"})
	for _, allPages := range []bool{false, true} {
		for _, allEndpoints := range []bool{false, true} {
			stored.AllPages = allPages
			stored.AllEndpoints = allEndpoints
			current := catalog.EffectiveGrants(stored)
			if slices.Contains(current.Pages, "/new-page") != allPages ||
				slices.Contains(current.Endpoints, "GET /new-endpoint") != allEndpoints {
				t.Fatalf("new grants did not follow independent modes: pages=%t endpoints=%t", allPages, allEndpoints)
			}
		}
	}
}
