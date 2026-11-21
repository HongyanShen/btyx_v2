package task

import (
	"testing"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestAggregatePlatformOutcomes(t *testing.T) {
	cases := []struct {
		name        string
		submissions []*ent.Submission
		expected    enum.AssignmentStatus
	}{
		{
			name: "rejected platform takes precedence over another pending platform",
			submissions: []*ent.Submission{
				{
					Platform: enum.PlatformDouyin,
					Status:   enum.SubmissionRejected,
				},
				{
					Platform: enum.PlatformXiaohongshu,
					Status:   enum.SubmissionPending,
				},
			},
			expected: enum.AssignmentRejected,
		},
		{
			name: "reissued platform does not block settlement of valid work",
			submissions: []*ent.Submission{
				{
					Platform:     enum.PlatformDouyin,
					Status:       enum.SubmissionApproved,
					SettlementID: "batch",
				},
				{
					Platform: enum.PlatformXiaohongshu,
					Status:   enum.SubmissionInvalid,
				},
			},
			expected: enum.AssignmentSettled,
		},
		{
			name: "all invalid platforms have no payable reward",
			submissions: []*ent.Submission{
				{
					Platform: enum.PlatformDouyin,
					Status:   enum.SubmissionInvalid,
				},
				{
					Platform: enum.PlatformXiaohongshu,
					Status:   enum.SubmissionInvalid,
				},
			},
			expected: enum.AssignmentNotPayable,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := aggregateStatus(
				[]enum.Platform{enum.PlatformDouyin, enum.PlatformXiaohongshu},
				test.submissions,
			)
			if actual != test.expected {
				t.Fatalf("got %s, want %s", actual, test.expected)
			}
		})
	}
}
