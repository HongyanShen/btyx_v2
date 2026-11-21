package enum

// AssignmentStatus defines the allowed wire values for this business concept.
type AssignmentStatus string

const (
	AssignmentInProgress        AssignmentStatus = "in_progress"
	AssignmentPendingReview     AssignmentStatus = "pending_review"
	AssignmentRejected          AssignmentStatus = "rejected"
	AssignmentPendingSettlement AssignmentStatus = "pending_settlement"
	AssignmentSettled           AssignmentStatus = "settled"
	AssignmentRevoked           AssignmentStatus = "revoked"
	AssignmentExpired           AssignmentStatus = "expired"
	AssignmentNotPayable        AssignmentStatus = "not_payable"
)

func (AssignmentStatus) Values() []string {
	return []string{
		string(AssignmentInProgress),
		string(AssignmentPendingReview),
		string(AssignmentRejected),
		string(AssignmentPendingSettlement),
		string(AssignmentSettled),
		string(AssignmentRevoked),
		string(AssignmentExpired),
		string(AssignmentNotPayable),
	}
}

func (value AssignmentStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *AssignmentStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (AssignmentStatus("")).Values())
}
