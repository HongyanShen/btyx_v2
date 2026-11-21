package enum

// EnrollmentStatus defines the allowed wire values for this business concept.
type EnrollmentStatus string

const (
	EnrollmentNone     EnrollmentStatus = "none"
	EnrollmentPending  EnrollmentStatus = "pending"
	EnrollmentApproved EnrollmentStatus = "approved"
	EnrollmentRejected EnrollmentStatus = "rejected"
)

func (EnrollmentStatus) Values() []string {
	return []string{
		string(EnrollmentNone),
		string(EnrollmentPending),
		string(EnrollmentApproved),
		string(EnrollmentRejected),
	}
}

func (value EnrollmentStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *EnrollmentStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (EnrollmentStatus("")).Values())
}
