package settlement

type SettlementInput struct {
	Through string `json:"through"`
	Key     string `json:"key"`
}

type BatchDTO struct {
	ID        string `json:"id"`
	Through   string `json:"through"`
	AmountFen string `json:"amount_fen"`
	Count     int64  `json:"count"`
	CreatorID string `json:"creator_id"`
}
