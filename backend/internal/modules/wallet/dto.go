package wallet

import (
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

type WalletDTO struct {
	AvailableFen     string `json:"available_fen"`
	FrozenFen        string `json:"frozen_fen"`
	WithdrawnFen     string `json:"withdrawn_fen"`
	EarnedFen        string `json:"earned_fen"`
	MonthEarnedFen   string `json:"month_earned_fen"`
	PendingFen       string `json:"pending_fen"`
	MinWithdrawalFen string `json:"min_withdrawal_fen"`
	MaxWithdrawalFen string `json:"max_withdrawal_fen"`
}

type WithdrawalDTO struct {
	ID          string                `json:"id"`
	UserID      string                `json:"user_id"`
	AmountFen   string                `json:"amount_fen"`
	Status      enum.WithdrawalStatus `json:"status"`
	Reason      string                `json:"reason"`
	PackageInfo string                `json:"package_info"`
	MerchantID  string                `json:"merchant_id"`
	CreatedAt   time.Time             `json:"created_at"`
}

type LedgerDTO struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Kind        enum.LedgerKind `json:"kind"`
	AmountFen   string          `json:"amount_fen"`
	ReferenceID string          `json:"reference_id"`
	OriginalID  string          `json:"original_id"`
	Reason      string          `json:"reason"`
	CreatedAt   time.Time       `json:"created_at"`
}
