package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/models"
)

type UserRepository interface {
	UpsertGoogleUser(
		ctx context.Context,
		identity domain.GoogleIdentity,
		role domain.UserRole,
		loginAt time.Time,
	) (*domain.User, error)
	GetByID(ctx context.Context, id uint) (*domain.User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) (*domain.Session, error)
	GetActiveByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error)
	RevokeByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error
	CleanupExpiredOrRevoked(ctx context.Context, now time.Time) (int64, error)
}

type userRepository struct {
	db *sqlx.DB
}

type sessionRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func NewSessionRepository(db *sqlx.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *userRepository) UpsertGoogleUser(
	ctx context.Context,
	identity domain.GoogleIdentity,
	role domain.UserRole,
	loginAt time.Time,
) (*domain.User, error) {
	query := `--sql
		INSERT INTO users (google_subject, email, display_name, avatar_url, role, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (google_subject)
		DO UPDATE SET
			email = EXCLUDED.email,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			role = EXCLUDED.role,
			last_login_at = EXCLUDED.last_login_at,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, google_subject, email, display_name, avatar_url, role, created_at, updated_at, last_login_at
	`
	var user models.User
	err := r.db.GetContext(
		ctx,
		&user,
		query,
		identity.Subject,
		identity.Email,
		identity.DisplayName,
		identity.AvatarURL,
		role,
		loginAt.UTC(),
	)
	if err != nil {
		return nil, ParseDBError(err)
	}
	return toDomainUser(&user), nil
}

func (r *userRepository) GetByID(ctx context.Context, id uint) (*domain.User, error) {
	query := `--sql
		SELECT id, google_subject, email, display_name, avatar_url, role, created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`
	var user models.User
	if err := r.db.GetContext(ctx, &user, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewErrNotFound("пользователь не найден")
		}
		return nil, ParseDBError(err)
	}
	return toDomainUser(&user), nil
}

func (r *sessionRepository) Create(ctx context.Context, session *domain.Session) (*domain.Session, error) {
	metadata, err := normalizeSessionMetadata(session.Metadata)
	if err != nil {
		return nil, err
	}
	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		return nil, domain.NewErrInvalidAuthData("metadata сессии должна быть валидным JSON")
	}

	query := `--sql
		INSERT INTO sessions (token_hash, user_id, expires_at, revoked_at, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err = r.db.QueryRowxContext(
		ctx,
		query,
		session.TokenHash,
		session.UserID,
		session.ExpiresAt.UTC(),
		session.RevokedAt,
		metadataRaw,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, ParseDBError(err)
	}
	session.CreatedAt = session.CreatedAt.UTC()
	session.UpdatedAt = session.UpdatedAt.UTC()
	session.Metadata = metadata
	return session, nil
}

func (r *sessionRepository) GetActiveByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*domain.Session, error) {
	query := `--sql
		SELECT id, token_hash, user_id, expires_at, revoked_at, metadata, created_at, updated_at
		FROM sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
	`
	var session models.Session
	if err := r.db.GetContext(ctx, &session, query, tokenHash, now.UTC()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewErrNotFound("сессия не найдена")
		}
		return nil, ParseDBError(err)
	}
	return toDomainSession(&session)
}

func (r *sessionRepository) RevokeByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	query := `--sql
		UPDATE sessions
		SET revoked_at = $2, updated_at = CURRENT_TIMESTAMP
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
	`
	result, err := r.db.ExecContext(ctx, query, tokenHash, revokedAt.UTC())
	if err != nil {
		return ParseDBError(err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения количества обновленных сессий: %w", err)
	}
	if rowsAffected == 0 {
		return domain.NewErrNotFound("сессия не найдена")
	}
	return nil
}

func (r *sessionRepository) CleanupExpiredOrRevoked(ctx context.Context, now time.Time) (int64, error) {
	query := `--sql
		DELETE FROM sessions
		WHERE expires_at <= $1 OR revoked_at IS NOT NULL
	`
	result, err := r.db.ExecContext(ctx, query, now.UTC())
	if err != nil {
		return 0, ParseDBError(err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ошибка получения количества очищенных сессий: %w", err)
	}
	return rowsAffected, nil
}

func toDomainUser(user *models.User) *domain.User {
	return &domain.User{
		ID:            user.ID,
		GoogleSubject: user.GoogleSubject,
		Email:         user.Email,
		DisplayName:   user.DisplayName,
		AvatarURL:     user.AvatarURL,
		Role:          domain.UserRole(user.Role),
		CreatedAt:     user.CreatedAt.UTC(),
		UpdatedAt:     user.UpdatedAt.UTC(),
		LastLoginAt:   user.LastLoginAt.UTC(),
	}
}

func toDomainSession(session *models.Session) (*domain.Session, error) {
	metadata := map[string]any{}
	if len(session.Metadata) > 0 {
		if err := json.Unmarshal(session.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("ошибка чтения metadata сессии: %w", err)
		}
	}
	return &domain.Session{
		ID:        session.ID,
		TokenHash: session.TokenHash,
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt.UTC(),
		RevokedAt: session.RevokedAt,
		Metadata:  metadata,
		CreatedAt: session.CreatedAt.UTC(),
		UpdatedAt: session.UpdatedAt.UTC(),
	}, nil
}

func normalizeSessionMetadata(metadata map[string]any) (map[string]any, error) {
	if metadata == nil {
		return map[string]any{}, nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, domain.NewErrInvalidAuthData("metadata сессии должна быть валидным JSON")
	}
	var normalized map[string]any
	if err = json.Unmarshal(encoded, &normalized); err != nil {
		return nil, domain.NewErrInvalidAuthData("metadata сессии должна быть валидным JSON")
	}
	for key := range normalized {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			return nil, domain.NewErrInvalidAuthData("metadata сессии содержит пустой ключ")
		}
	}
	return normalized, nil
}
