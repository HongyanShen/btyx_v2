package enum

// AssignmentSource defines the allowed wire values for this business concept.
type AssignmentSource string

const (
	SourceClaim    AssignmentSource = "claim"
	SourceAssigned AssignmentSource = "assigned"
	SourceReissued AssignmentSource = "reissued"
)

func (AssignmentSource) Values() []string {
	return []string{
		string(SourceClaim),
		string(SourceAssigned),
		string(SourceReissued),
	}
}

func (value AssignmentSource) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *AssignmentSource) UnmarshalJSON(data []byte) error {
	return decode(data, value, (AssignmentSource("")).Values())
}
