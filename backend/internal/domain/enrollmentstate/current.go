package enrollmentstate

import (
	"baituber.cn/yanxuan/v2/ent"
	"baituber.cn/yanxuan/v2/ent/userenrollment"
)

func LoadCurrent(query *ent.UserEnrollmentQuery) {
	query.Where(userenrollment.IsCurrentEQ(true))
}

func Current(member *ent.User) *ent.UserEnrollment {
	for _, record := range member.Edges.Enrollment {
		if record.IsCurrent {
			return record
		}
	}

	return nil
}
