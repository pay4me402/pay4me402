package users

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/majed/payformeproxy/internal/db"
	"golang.org/x/crypto/argon2"
)

const (
	hashMemory      = 64 * 1024
	hashIterations  = 3
	hashParallelism = 2
	hashKeyLength   = 32
	saltLength      = 16
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) Create(ctx context.Context, username string, password string) (db.ProxyUser, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return db.ProxyUser{}, errors.New("username is required")
	}
	if password == "" {
		return db.ProxyUser{}, errors.New("password is required")
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return db.ProxyUser{}, err
	}

	return s.queries.CreateProxyUser(ctx, db.CreateProxyUserParams{
		ID:           newID(),
		Username:     username,
		PasswordHash: passwordHash,
	})
}

func (s *Service) Authenticate(ctx context.Context, username string, password string) (bool, error) {
	user, err := s.AuthenticateUser(ctx, username, password)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return user.ID != "", err
}

func (s *Service) AuthenticateUser(ctx context.Context, username string, password string) (db.ProxyUser, error) {
	user, err := s.queries.GetProxyUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return db.ProxyUser{}, nil
	}
	if err != nil {
		return db.ProxyUser{}, err
	}
	valid, err := verifyPassword(password, user.PasswordHash)
	if err != nil {
		return db.ProxyUser{}, err
	}
	if !valid {
		return db.ProxyUser{}, nil
	}
	return user, nil
}

func (s *Service) List(ctx context.Context) ([]db.ProxyUser, error) {
	return s.queries.ListProxyUsers(ctx)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.queries.DeleteProxyUser(ctx, id)
}

func newID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, hashIterations, hashMemory, hashParallelism, hashKeyLength)
	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		hashMemory,
		hashIterations,
		hashParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func verifyPassword(password string, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false, errors.New("unsupported password hash format")
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("parse password hash params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}

	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	if len(actual) != len(expected) {
		return false, nil
	}

	var diff byte
	for i := range actual {
		diff |= actual[i] ^ expected[i]
	}
	return diff == 0, nil
}
