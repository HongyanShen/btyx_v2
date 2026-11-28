package audit

import (
	"slices"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

// Routes exclusive to one platform before audit events stored their HTTP origin.
// Shared or unrecorded endpoints cannot establish the source of a historical event.
var historicalEndpoints = map[enum.AuditPlatform][]string{
	enum.AuditPlatformMiniapp: {
		"PUT /api/v1/me",
		"POST /api/v1/me/phone",
		"POST /api/v1/me/identity",
		"POST /api/v1/enrollment",
		"POST /api/v1/agreement/consent",
		"POST /api/v1/tasks/claim",
		"POST /api/v1/assignments/:id/submissions",
		"POST /api/v1/withdrawals",
		"POST /api/v1/creative/tasks/:id/claim",
		"POST /api/v1/creative/works/:id/submit",
		"POST /api/v1/creative/works/:id/links",
		"POST /api/v1/creative/works/:id/evidence",
	},
	enum.AuditPlatformAdmin: {
		"POST /profile/password",
		"POST /agreements",
		"POST /operators",
		"POST /operators/:id",
		"POST /feedback/:id/reply",
		"POST /ledger/:id/adjust",
		"POST /contents",
		"POST /contents/:id/tags",
		"POST /users/manage/:id/block",
		"POST /users/:id/review",
		"POST /users/tags",
		"POST /users/tags/:id/:action",
		"POST /reviews/:id/:action",
		"POST /settlements",
		"POST /distribution/settings",
		"POST /distribution/settings/marketing",
		"POST /tasks",
		"POST /tasks/settings",
		"POST /tasks/:id/:action",
		"POST /api/v1/operators",
		"POST /api/v1/operators/:id",
		"POST /api/v1/users/:id/review",
		"POST /api/v1/users/:id/block",
		"POST /api/v1/marketing-tags",
		"POST /api/v1/marketing-tags/:id/rename",
		"POST /api/v1/marketing-tags/:id/delete",
		"POST /api/v1/marketing-settings",
		"POST /api/v1/tasks",
		"POST /api/v1/tasks/:id/versions",
		"POST /api/v1/tasks/:id/visibility",
		"POST /api/v1/tasks/:id/assign",
		"POST /api/v1/assignments/:id/revoke",
		"POST /api/v1/assignments/:id/review",
		"POST /api/v1/assignments/:id/reissue",
		"POST /api/v1/settlements",
		"POST /api/v1/distribution/settings",
		"POST /api/v1/ledger/:id/adjust",
		"POST /api/v1/contents",
		"POST /api/v1/contents/:id/tags",
		"POST /api/v1/feedback/:id/reply",
		"POST /api/v1/agreements",
		"POST /api/v1/creative/tasks",
		"POST /api/v1/creative/tasks/:id/visibility",
		"POST /api/v1/creative/works/:id/review",
		"POST /api/v1/creative/works/:id/return",
		"POST /api/v1/creative/works/:id/award",
	},
}

func recordedPlatform(platform enum.AuditPlatform, endpoint string) enum.AuditPlatform {
	if platform != "" {
		return platform
	}

	for source, endpoints := range historicalEndpoints {
		if slices.Contains(endpoints, endpoint) {
			return source
		}
	}

	return ""
}
