package enum

type CreativeWorkStatus string

const (
	CreativeDraft    CreativeWorkStatus = "draft"
	CreativePending  CreativeWorkStatus = "pending"
	CreativeRejected CreativeWorkStatus = "rejected"
	CreativeApproved CreativeWorkStatus = "approved"
	CreativeLinked   CreativeWorkStatus = "linked"
	CreativeRewarded CreativeWorkStatus = "rewarded"
)

func (CreativeWorkStatus) Values() []string {
	return []string{string(CreativeDraft), string(CreativePending), string(CreativeRejected), string(CreativeApproved), string(CreativeLinked), string(CreativeRewarded)}
}

func (value CreativeWorkStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *CreativeWorkStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (CreativeWorkStatus("")).Values())
}
