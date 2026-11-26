package service

import (
	"context"
	"fmt"
	"time"
)

type MonthlyCheckinCredit struct {
	Available float64   `json:"available"`
	Month     string    `json:"month"`
	ExpiresAt time.Time `json:"expires_at"`
}

type LegacyCheckinCredit struct {
	Available  float64   `json:"available"`
	NextAmount float64   `json:"next_amount"`
	NextExpiry time.Time `json:"next_expiry"`
}

type WalletBalance struct {
	AvailableBalance    float64              `json:"available_balance"`
	FrozenBalance       float64              `json:"frozen_balance"`
	OtherAvailable      float64              `json:"other_available"`
	CheckinCredit       MonthlyCheckinCredit `json:"checkin_credit"`
	LegacyCheckinCredit *LegacyCheckinCredit `json:"legacy_checkin_credit,omitempty"`
}

type WalletBalanceRepository interface {
	GetWalletBalance(ctx context.Context, userID int64) (WalletBalance, error)
}

func (s *UserService) GetWalletBalance(ctx context.Context, userID int64) (WalletBalance, error) {
	repo, ok := s.userRepo.(WalletBalanceRepository)
	if !ok {
		return WalletBalance{}, fmt.Errorf("wallet balance repository unavailable")
	}
	return repo.GetWalletBalance(ctx, userID)
}
