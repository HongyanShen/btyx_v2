package enum

// FeedbackStatus defines the allowed wire values for this business concept.
type FeedbackStatus string

const (
	FeedbackOpen    FeedbackStatus = "open"
	FeedbackReplied FeedbackStatus = "replied"
	FeedbackClosed  FeedbackStatus = "closed"
)

func (FeedbackStatus) Values() []string {
	return []string{
		string(FeedbackOpen),
		string(FeedbackReplied),
		string(FeedbackClosed),
	}
}

func (value FeedbackStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *FeedbackStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (FeedbackStatus("")).Values())
}
