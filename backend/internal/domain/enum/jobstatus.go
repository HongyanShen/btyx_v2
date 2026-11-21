package enum

// JobStatus defines the allowed wire values for this business concept.
type JobStatus string

const (
	JobQueued        JobStatus = "queued"
	JobRunning       JobStatus = "running"
	JobDone          JobStatus = "done"
	JobRetry         JobStatus = "retry"
	JobFailed        JobStatus = "failed"
	JobBlockedConfig JobStatus = "blocked_config"
)

func (JobStatus) Values() []string {
	return []string{
		string(JobQueued),
		string(JobRunning),
		string(JobDone),
		string(JobRetry),
		string(JobFailed),
		string(JobBlockedConfig),
	}
}

func (value JobStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *JobStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (JobStatus("")).Values())
}
