package enum

type SubscriptionResult string

const (
	SubscriptionAccept  SubscriptionResult = "accept"
	SubscriptionReject  SubscriptionResult = "reject"
	SubscriptionBan     SubscriptionResult = "ban"
	SubscriptionFilter  SubscriptionResult = "filter"
	SubscriptionError   SubscriptionResult = "error"
	SubscriptionUnknown SubscriptionResult = "unknown"
)

func (SubscriptionResult) Values() []string {
	return []string{string(SubscriptionAccept), string(SubscriptionReject), string(SubscriptionBan), string(SubscriptionFilter), string(SubscriptionError), string(SubscriptionUnknown)}
}

func (v SubscriptionResult) Valid() bool {
	return valid(string(v), v.Values())
}

func (v *SubscriptionResult) UnmarshalJSON(data []byte) error {
	return decode(data, v, (SubscriptionResult("")).Values())
}
