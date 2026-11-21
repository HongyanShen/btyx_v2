package enum

// JobKind defines the allowed wire values for this business concept.
type JobKind string

const (
	JobTransfer        JobKind = "transfer"
	JobTransferQuery   JobKind = "transfer_query"
	JobNotification    JobKind = "notification"
	JobVisibilityCheck JobKind = "visibility_check"
)

func (JobKind) Values() []string {
	return []string{
		string(JobTransfer),
		string(JobTransferQuery),
		string(JobNotification),
		string(JobVisibilityCheck),
	}
}

func (value JobKind) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *JobKind) UnmarshalJSON(data []byte) error {
	return decode(data, value, (JobKind("")).Values())
}
