package enum

// Visibility defines the allowed wire values for this business concept.
type Visibility string

const (
	VisibilityUnchecked      Visibility = "unchecked"
	VisibilityManualVerified Visibility = "manual_verified"
	VisibilityVisible        Visibility = "visible"
	VisibilityUnavailable    Visibility = "unavailable"
	VisibilityUnknown        Visibility = "unknown"
)

func (Visibility) Values() []string {
	return []string{
		string(VisibilityUnchecked),
		string(VisibilityManualVerified),
		string(VisibilityVisible),
		string(VisibilityUnavailable),
		string(VisibilityUnknown),
	}
}

func (value Visibility) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *Visibility) UnmarshalJSON(data []byte) error {
	return decode(data, value, (Visibility("")).Values())
}
