package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/repository"
)

const sessionTokenSize = 32

type AuthService interface {
	UpsertGoogleUser(ctx context.Context, identity domain.GoogleIdentity, loginAt time.Time) (*domain.User, error)
	CreateSession(
		ctx context.Context,
		userID uint,
		ttl time.Duration,
		now time.Time,
		metadata map[string]any,
	) (string, *domain.Session, error)
	GetActiveSessionByRawToken(ctx context.Context, rawToken string, now time.Time) (*domain.Session, error)
	RevokeSessionByRawToken(ctx context.Context, rawToken string, now time.Time) error
	CleanupExpiredOrRevokedSessions(ctx context.Context, now time.Time) (int64, error)
}

type authService struct {
	users       repository.UserRepository
	sessions    repository.SessionRepository
	adminEmails map[string]struct{}
}

func NewAuthService(
	users repository.UserRepository,
	sessions repository.SessionRepository,
	adminEmails []string,
) AuthService {
	normalizedAdmins := make(map[string]struct{}, len(adminEmails))
	for _, email := range adminEmails {
		normalized := normalizeEmail(email)
		if normalized != "" {
			normalizedAdmins[normalized] = struct{}{}
		}
	}
	return &authService{
		users:       users,
		sessions:    sessions,
		adminEmails: normalizedAdmins,
	}
}

func (s *authService) UpsertGoogleUser(
	ctx context.Context,
	identity domain.GoogleIdentity,
	loginAt time.Time,
) (*domain.User, error) {
	if strings.TrimSpace(identity.Subject) == "" {
		return nil, domain.NewErrInvalidAuthData("google subject обязателен")
	}
	email := normalizeEmail(identity.Email)
	if email == "" {
		return nil, domain.NewErrInvalidAuthData("email обязателен")
	}
	if strings.TrimSpace(identity.DisplayName) == "" {
		return nil, domain.NewErrInvalidAuthData("display_name обязателен")
	}

	identity.Email = email
	role := domain.UserRoleUser
	if _, ok := s.adminEmails[email]; ok {
		role = domain.UserRoleAdmin
	}
	return s.users.UpsertGoogleUser(ctx, identity, role, loginAt.UTC())
}

func (s *authService) CreateSession(
	ctx context.Context,
	userID uint,
	ttl time.Duration,
	now time.Time,
	metadata map[string]any,
) (string, *domain.Session, error) {
	if userID == 0 {
		return "", nil, domain.NewErrInvalidAuthData("user_id должен быть больше 0")
	}
	if ttl <= 0 {
		return "", nil, domain.NewErrInvalidAuthData("session ttl должен быть положительным")
	}

	rawToken, err := generateSessionToken()
	if err != nil {
		return "", nil, domain.NewErrServiceUnavailable("не удалось создать токен сессии")
	}

	session := &domain.Session{
		TokenHash: hashToken(rawToken),
		UserID:    userID,
		ExpiresAt: now.UTC().Add(ttl),
		Metadata:  metadata,
	}
	created, err := s.sessions.Create(ctx, session)
	if err != nil {
		return "", nil, err
	}
	return rawToken, created, nil
}

func (s *authService) GetActiveSessionByRawToken(
	ctx context.Context,
	rawToken string,
	now time.Time,
) (*domain.Session, error) {
	hash, err := tokenHashFromRaw(rawToken)
	if err != nil {
		return nil, err
	}

	session, err := s.sessions.GetActiveByTokenHash(ctx, hash, now.UTC())
	if err != nil {
		if isNotFoundError(err) {
			return nil, domain.NewErrUnauthorized("сессия недействительна")
		}
		return nil, err
	}
	return session, nil
}

func (s *authService) RevokeSessionByRawToken(ctx context.Context, rawToken string, now time.Time) error {
	hash, err := tokenHashFromRaw(rawToken)
	if err != nil {
		return err
	}

	err = s.sessions.RevokeByTokenHash(ctx, hash, now.UTC())
	if err != nil {
		if isNotFoundError(err) {
			return domain.NewErrUnauthorized("сессия недействительна")
		}
		return err
	}
	return nil
}

func (s *authService) CleanupExpiredOrRevokedSessions(ctx context.Context, now time.Time) (int64, error) {
	return s.sessions.CleanupExpiredOrRevoked(ctx, now.UTC())
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func generateSessionToken() (string, error) {
	buf := make([]byte, sessionTokenSize)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenHashFromRaw(rawToken string) (string, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", domain.NewErrUnauthorized("сессия отсутствует")
	}
	return hashToken(token), nil
}

func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func isNotFoundError(err error) bool {
	return errors.Is(err, domain.ErrNotFound)
}
