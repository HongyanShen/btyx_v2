package enum

type EnrollmentAge string

const (
	EnrollmentAge18To24  EnrollmentAge = "18_24"
	EnrollmentAge25To30  EnrollmentAge = "25_30"
	EnrollmentAge31To40  EnrollmentAge = "31_40"
	EnrollmentAgeOver40  EnrollmentAge = "over_40"
	EnrollmentAgeUnknown EnrollmentAge = "unknown"
)

func (EnrollmentAge) Values() []string {
	return []string{
		string(EnrollmentAge18To24),
		string(EnrollmentAge25To30),
		string(EnrollmentAge31To40),
		string(EnrollmentAgeOver40),
		string(EnrollmentAgeUnknown),
	}
}

func (value EnrollmentAge) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *EnrollmentAge) UnmarshalJSON(data []byte) error {
	return decode(data, value, (EnrollmentAge("")).Values())
}
