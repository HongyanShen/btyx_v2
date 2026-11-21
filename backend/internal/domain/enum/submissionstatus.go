package enum

// SubmissionStatus defines the allowed wire values for this business concept.
type SubmissionStatus string

const (
	SubmissionPending  SubmissionStatus = "pending"
	SubmissionApproved SubmissionStatus = "approved"
	SubmissionRejected SubmissionStatus = "rejected"
	SubmissionInvalid  SubmissionStatus = "invalid"
	SubmissionWaived   SubmissionStatus = "waived"
)

func (SubmissionStatus) Values() []string {
	return []string{
		string(SubmissionPending),
		string(SubmissionApproved),
		string(SubmissionRejected),
		string(SubmissionInvalid),
		string(SubmissionWaived),
	}
}

func (value SubmissionStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *SubmissionStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (SubmissionStatus("")).Values())
}
