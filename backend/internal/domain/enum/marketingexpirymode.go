package enum

type MarketingExpiryMode string

const (
	MarketingExpiryFixedDuration MarketingExpiryMode = "fixed_duration"
	MarketingExpiryEndOfDay      MarketingExpiryMode = "end_of_day"
)

func (MarketingExpiryMode) Values() []string {
	return []string{string(MarketingExpiryFixedDuration), string(MarketingExpiryEndOfDay)}
}

func (value MarketingExpiryMode) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *MarketingExpiryMode) UnmarshalJSON(data []byte) error {
	return decode(data, value, (MarketingExpiryMode("")).Values())
}
