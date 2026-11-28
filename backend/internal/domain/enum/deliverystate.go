package enum

type DeliveryState string

const (
	DeliveryStation DeliveryState = "station_only"
	DeliveryPending DeliveryState = "pending"
	DeliverySending DeliveryState = "sending"
	DeliverySent    DeliveryState = "sent"
	DeliverySkipped DeliveryState = "skipped"
	DeliveryFailed  DeliveryState = "failed"
	DeliveryUnknown DeliveryState = "unknown"
)

func (DeliveryState) Values() []string {
	return []string{string(DeliveryStation), string(DeliveryPending), string(DeliverySending), string(DeliverySent), string(DeliverySkipped), string(DeliveryFailed), string(DeliveryUnknown)}
}

func (v DeliveryState) Valid() bool {
	return valid(string(v), v.Values())
}

func (v *DeliveryState) UnmarshalJSON(data []byte) error {
	return decode(data, v, (DeliveryState("")).Values())
}
