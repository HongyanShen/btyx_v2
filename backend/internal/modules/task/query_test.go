package task

import (
	"testing"
	"time"

	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

func TestAssignmentDetailUsesFirstLinkDateForPublication(t *testing.T) {
	assignment := &ent.Assignment{
		BusinessDate: "2026-11-22",
		Platforms:    []enum.Platform{enum.PlatformDouyin},
	}
	link := &ent.Submission{
		Platform:  enum.PlatformDouyin,
		CreatedAt: time.Date(2026, 11, 22, 16, 15, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 11, 24, 1, 0, 0, 0, time.UTC),
	}

	detail := assignmentDetail(assignment, &ent.TaskVersion{}, []*ent.Submission{link})
	if got := detail.Submissions[0].PublishedOn; got != "2026-11-23" {
		t.Fatalf("publication date = %q, want 2026-11-23", got)
	}
}
