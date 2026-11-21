package enum

type WechatTransferState string

const (
	WechatTransferAccepted     WechatTransferState = "ACCEPTED"
	WechatTransferProcessing   WechatTransferState = "PROCESSING"
	WechatTransferWaitConfirm  WechatTransferState = "WAIT_USER_CONFIRM"
	WechatTransferTransferring WechatTransferState = "TRANSFERING"
	WechatTransferCancelling   WechatTransferState = "CANCELING"
	WechatTransferSuccess      WechatTransferState = "SUCCESS"
	WechatTransferFailed       WechatTransferState = "FAIL"
	WechatTransferCancelled    WechatTransferState = "CANCELLED"
)

func (WechatTransferState) Values() []string {
	return []string{
		string(WechatTransferAccepted),
		string(WechatTransferProcessing),
		string(WechatTransferWaitConfirm),
		string(WechatTransferTransferring),
		string(WechatTransferCancelling),
		string(WechatTransferSuccess),
		string(WechatTransferFailed),
		string(WechatTransferCancelled),
	}
}

func (value WechatTransferState) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *WechatTransferState) UnmarshalJSON(data []byte) error {
	return decode(data, value, (WechatTransferState("")).Values())
}
