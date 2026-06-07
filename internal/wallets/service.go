package wallets

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/majed/payformeproxy/internal/db"
)

const ChainAlgorand = "algorand"
const ChainSolana = "solana"
const DefaultAlgorandRPCEndpoint = "https://testnet-api.algonode.cloud"
const DefaultSolanaRPCEndpoint = "https://api.mainnet-beta.solana.com"

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) Create(ctx context.Context, name string, chain string, privateKey string, rpcEndpoint string, rpcToken string) (db.Wallet, error) {
	name = strings.TrimSpace(name)
	chain = strings.TrimSpace(strings.ToLower(chain))
	privateKey = strings.TrimSpace(privateKey)
	rpcEndpoint = strings.TrimSpace(rpcEndpoint)
	rpcToken = strings.TrimSpace(rpcToken)
	if name == "" {
		return db.Wallet{}, errors.New("wallet name is required")
	}
	if chain != ChainAlgorand && chain != ChainSolana {
		return db.Wallet{}, errors.New("wallet chain must be algorand or solana")
	}
	if privateKey == "" {
		return db.Wallet{}, errors.New("wallet private key is required")
	}
	if rpcEndpoint == "" {
		rpcEndpoint = DefaultRPCEndpoint(chain)
	}

	return s.queries.CreateWallet(ctx, db.CreateWalletParams{
		ID:          newID(),
		Name:        name,
		Chain:       chain,
		PrivateKey:  privateKey,
		RpcEndpoint: rpcEndpoint,
		RpcToken: sql.NullString{
			String: rpcToken,
			Valid:  rpcToken != "",
		},
	})
}

func DefaultRPCEndpoint(chain string) string {
	switch chain {
	case ChainAlgorand:
		return DefaultAlgorandRPCEndpoint
	case ChainSolana:
		return DefaultSolanaRPCEndpoint
	default:
		return ""
	}
}

func (s *Service) List(ctx context.Context) ([]db.Wallet, error) {
	return s.queries.ListWallets(ctx)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.queries.DeleteWallet(ctx, id)
}

func (s *Service) PrivateKeyForChain(ctx context.Context, chain string) (string, error) {
	wallet, err := s.WalletForChain(ctx, chain)
	if err != nil {
		return "", err
	}
	if wallet.ID == "" {
		return "", nil
	}
	return wallet.PrivateKey, nil
}

func (s *Service) WalletForChain(ctx context.Context, chain string) (db.Wallet, error) {
	wallet, err := s.queries.GetWalletByChain(ctx, chain)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Wallet{}, nil
	}
	if err != nil {
		return db.Wallet{}, err
	}
	return wallet, nil
}

func newID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}
