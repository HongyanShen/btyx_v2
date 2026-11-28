package enum

type SettlementFrequency string

const (
	SettlementWeekly     SettlementFrequency = "weekly"
	SettlementDaily      SettlementFrequency = "daily"
	SettlementMonthly    SettlementFrequency = "monthly"
	SettlementAfterDelay SettlementFrequency = "after_delay"
)

func (SettlementFrequency) Values() []string {
	return []string{
		string(SettlementWeekly),
		string(SettlementDaily),
		string(SettlementMonthly),
		string(SettlementAfterDelay),
	}
}

func (value SettlementFrequency) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *SettlementFrequency) UnmarshalJSON(data []byte) error {
	return decode(data, value, (SettlementFrequency("")).Values())
}
