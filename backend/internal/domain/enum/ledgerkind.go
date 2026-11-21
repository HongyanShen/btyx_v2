package enum

// LedgerKind defines the allowed wire values for this business concept.
type LedgerKind string

const (
	LedgerSettlementCredit  LedgerKind = "settlement_credit"
	LedgerWithdrawalFreeze  LedgerKind = "withdrawal_freeze"
	LedgerWithdrawalSuccess LedgerKind = "withdrawal_success"
	LedgerWithdrawalRefund  LedgerKind = "withdrawal_refund"
	LedgerAdjustment        LedgerKind = "adjustment"
)

func (LedgerKind) Values() []string {
	return []string{
		string(LedgerSettlementCredit),
		string(LedgerWithdrawalFreeze),
		string(LedgerWithdrawalSuccess),
		string(LedgerWithdrawalRefund),
		string(LedgerAdjustment),
	}
}

func (value LedgerKind) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *LedgerKind) UnmarshalJSON(data []byte) error {
	return decode(data, value, (LedgerKind("")).Values())
}
