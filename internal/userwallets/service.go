package userwallets

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/majed/payformeproxy/internal/db"
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) Assign(ctx context.Context, userID string, walletID string, monthlyBudget *float64) (db.UserWallet, error) {
	userID = strings.TrimSpace(userID)
	walletID = strings.TrimSpace(walletID)
	if userID == "" {
		return db.UserWallet{}, errors.New("user id is required")
	}
	if walletID == "" {
		return db.UserWallet{}, errors.New("wallet id is required")
	}

	return s.queries.CreateUserWallet(ctx, db.CreateUserWalletParams{
		ID:            newID(),
		UserID:        userID,
		WalletID:      walletID,
		MonthlyBudget: nullableMonthlyBudget(monthlyBudget),
	})
}

func (s *Service) List(ctx context.Context) ([]db.UserWallet, error) {
	return s.queries.ListUserWallets(ctx)
}

func (s *Service) ListByUser(ctx context.Context, userID string) ([]db.UserWallet, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user id is required")
	}
	return s.queries.ListUserWalletsByUser(ctx, userID)
}

func (s *Service) ListByWallet(ctx context.Context, walletID string) ([]db.UserWallet, error) {
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return nil, errors.New("wallet id is required")
	}
	return s.queries.ListUserWalletsByWallet(ctx, walletID)
}

func (s *Service) Get(ctx context.Context, id string) (db.UserWallet, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return db.UserWallet{}, errors.New("user wallet id is required")
	}
	userWallet, err := s.queries.GetUserWallet(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.UserWallet{}, nil
	}
	return userWallet, err
}

func (s *Service) UpdateMonthlyBudget(ctx context.Context, id string, monthlyBudget *float64) (db.UserWallet, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return db.UserWallet{}, errors.New("user wallet id is required")
	}
	return s.queries.UpdateUserWalletMonthlyBudget(ctx, db.UpdateUserWalletMonthlyBudgetParams{
		ID:            id,
		MonthlyBudget: nullableMonthlyBudget(monthlyBudget),
	})
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("user wallet id is required")
	}
	return s.queries.DeleteUserWallet(ctx, id)
}

func nullableMonthlyBudget(monthlyBudget *float64) sql.NullFloat64 {
	if monthlyBudget == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *monthlyBudget, Valid: true}
}

func newID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}
