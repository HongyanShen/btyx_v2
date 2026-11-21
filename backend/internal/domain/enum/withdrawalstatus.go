package enum

// WithdrawalStatus defines the allowed wire values for this business concept.
type WithdrawalStatus string

const (
	WithdrawalQueued              WithdrawalStatus = "queued"
	WithdrawalSubmitting          WithdrawalStatus = "submitting"
	WithdrawalUnknown             WithdrawalStatus = "unknown"
	WithdrawalWaitingConfirmation WithdrawalStatus = "waiting_confirmation"
	WithdrawalSuccess             WithdrawalStatus = "success"
	WithdrawalFailed              WithdrawalStatus = "failed"
	WithdrawalCancelled           WithdrawalStatus = "cancelled"
	WithdrawalBlockedConfig       WithdrawalStatus = "blocked_config"
)

func (WithdrawalStatus) Values() []string {
	return []string{
		string(WithdrawalQueued),
		string(WithdrawalSubmitting),
		string(WithdrawalUnknown),
		string(WithdrawalWaitingConfirmation),
		string(WithdrawalSuccess),
		string(WithdrawalFailed),
		string(WithdrawalCancelled),
		string(WithdrawalBlockedConfig),
	}
}

func (value WithdrawalStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *WithdrawalStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (WithdrawalStatus("")).Values())
}
