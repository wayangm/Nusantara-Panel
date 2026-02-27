package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"nusantara/internal/idgen"
	"nusantara/internal/security/password"
	"nusantara/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidPassword    = errors.New("invalid current password")
)

type Service struct {
	repo     store.Repository
	tokenTTL time.Duration
}

func NewService(repo store.Repository, tokenTTL time.Duration) *Service {
	return &Service{
		repo:     repo,
		tokenTTL: tokenTTL,
	}
}

func (s *Service) EnsureBootstrapAdmin(ctx context.Context, username, plainPassword string) error {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if strings.TrimSpace(plainPassword) == "" {
		return errors.New("bootstrap admin password is empty")
	}

	hash, err := password.Hash(plainPassword)
	if err != nil {
		return err
	}
	id, err := idgen.New("usr")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.repo.CreateUser(ctx, store.User{
		ID:           id,
		Username:     strings.TrimSpace(username),
		PasswordHash: hash,
		Role:         store.RoleAdmin,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

func (s *Service) Login(ctx context.Context, username, plainPassword string) (string, time.Time, store.User, error) {
	user, err := s.repo.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", time.Time{}, store.User{}, ErrInvalidCredentials
		}
		return "", time.Time{}, store.User{}, err
	}
	if !user.IsActive || !password.Verify(plainPassword, user.PasswordHash) {
		return "", time.Time{}, store.User{}, ErrInvalidCredentials
	}

	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, store.User{}, err
	}
	expiresAt := time.Now().UTC().Add(s.tokenTTL)
	if err := s.repo.CreateSession(ctx, store.Session{
		TokenHash: hashToken(token),
		UserID:    user.ID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return "", time.Time{}, store.User{}, err
	}

	user.PasswordHash = ""
	return token, expiresAt, user, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (store.User, error) {
	if token == "" {
		return store.User{}, ErrUnauthorized
	}
	session, err := s.repo.GetSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.User{}, ErrUnauthorized
		}
		return store.User{}, err
	}
	if session.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.repo.DeleteSessionByTokenHash(ctx, hashToken(token))
		return store.User{}, ErrUnauthorized
	}
	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.User{}, ErrUnauthorized
		}
		return store.User{}, err
	}
	if !user.IsActive {
		return store.User{}, ErrUnauthorized
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return ErrUnauthorized
	}
	return s.repo.DeleteSessionByTokenHash(ctx, hashToken(token))
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	if !password.Verify(currentPassword, user.PasswordHash) {
		return ErrInvalidPassword
	}
	newHash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdateUserPassword(ctx, user.ID, newHash, time.Now().UTC())
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

