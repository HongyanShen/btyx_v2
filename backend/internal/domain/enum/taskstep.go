package enum

// TaskStep defines the allowed wire values for this business concept.
type TaskStep string

const (
	StepContent TaskStep = "content"
	StepPublish TaskStep = "publish"
	StepLinks   TaskStep = "links"
	StepData    TaskStep = "data"
	StepSettle  TaskStep = "settle"
)

func (TaskStep) Values() []string {
	return []string{
		string(StepContent),
		string(StepPublish),
		string(StepLinks),
		string(StepData),
		string(StepSettle),
	}
}

func (value TaskStep) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *TaskStep) UnmarshalJSON(data []byte) error {
	return decode(data, value, (TaskStep("")).Values())
}
