package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type AuthRepositoryTestSuite struct {
	suite.Suite
	pgContainer *testutil.PostgresContainer
	users       UserRepository
	sessions    SessionRepository
	ctx         context.Context
}

func (suite *AuthRepositoryTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	pgContainer, err := testutil.SetupContainerAndMigrations(suite.ctx)
	if err != nil {
		suite.T().Fatalf("Не удалось подготовить тестовую БД: %v", err)
	}
	suite.pgContainer = pgContainer
	suite.users = NewUserRepository(suite.pgContainer.DB)
	suite.sessions = NewSessionRepository(suite.pgContainer.DB)
}

func (suite *AuthRepositoryTestSuite) TearDownTest() {
	if err := testutil.TruncateAuthTables(suite.pgContainer.DB); err != nil {
		suite.T().Fatalf("Не удалось очистить auth таблицы: %v", err)
	}
}

func (suite *AuthRepositoryTestSuite) TearDownSuite() {
	if suite.pgContainer == nil {
		return
	}
	if suite.pgContainer.DB != nil {
		_ = suite.pgContainer.DB.Close()
	}
	if err := suite.pgContainer.Terminate(suite.ctx); err != nil {
		suite.T().Fatalf("Не удалось завершить контейнер postgres: %s", err)
	}
}

func (suite *AuthRepositoryTestSuite) TestUpsertGoogleUser_CreateAndUpdate() {
	t := suite.T()
	now := time.Now().UTC().Add(-time.Hour)
	user, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "user@example.com",
			DisplayName: "User One",
			AvatarURL:   "https://example.com/avatar-1.png",
		},
		domain.UserRoleUser,
		now,
	)
	assert.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.Equal(t, domain.UserRoleUser, user.Role)

	updated, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "admin@example.com",
			DisplayName: "Admin User",
			AvatarURL:   "https://example.com/avatar-2.png",
		},
		domain.UserRoleAdmin,
		now.Add(30*time.Minute),
	)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, updated.ID)
	assert.Equal(t, domain.UserRoleAdmin, updated.Role)
	assert.Equal(t, "admin@example.com", updated.Email)
	assert.Equal(t, "Admin User", updated.DisplayName)
}

func (suite *AuthRepositoryTestSuite) TestUpsertGoogleUser_DuplicateEmail() {
	t := suite.T()
	_, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "same@example.com",
			DisplayName: "User One",
			AvatarURL:   "",
		},
		domain.UserRoleUser,
		time.Now(),
	)
	assert.NoError(t, err)

	_, err = suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-2",
			Email:       "SAME@example.com",
			DisplayName: "User Two",
			AvatarURL:   "",
		},
		domain.UserRoleUser,
		time.Now(),
	)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAlreadyExists))
}

func (suite *AuthRepositoryTestSuite) TestGetByID() {
	t := suite.T()
	now := time.Now().UTC().Add(-time.Hour)
	created, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "user@example.com",
			DisplayName: "User One",
			AvatarURL:   "https://example.com/avatar-1.png",
		},
		domain.UserRoleUser,
		now,
	)
	assert.NoError(t, err)
	assert.NotZero(t, created.ID)

	got, err := suite.users.GetByID(suite.ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "google-subject-1", got.GoogleSubject)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "User One", got.DisplayName)
	assert.Equal(t, "https://example.com/avatar-1.png", got.AvatarURL)
	assert.Equal(t, domain.UserRoleUser, got.Role)
	assert.Equal(t, created.CreatedAt, got.CreatedAt)
	assert.Equal(t, created.UpdatedAt, got.UpdatedAt)
	assert.True(t, got.LastLoginAt.Equal(created.LastLoginAt))
	assert.Equal(t, time.UTC, got.CreatedAt.Location())
	assert.Equal(t, time.UTC, got.LastLoginAt.Location())

	updated, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "admin@example.com",
			DisplayName: "Admin User",
			AvatarURL:   "https://example.com/avatar-2.png",
		},
		domain.UserRoleAdmin,
		now.Add(30*time.Minute),
	)
	assert.NoError(t, err)

	got, err = suite.users.GetByID(suite.ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, updated.Email, got.Email)
	assert.Equal(t, "Admin User", got.DisplayName)
	assert.Equal(t, "https://example.com/avatar-2.png", got.AvatarURL)
	assert.Equal(t, domain.UserRoleAdmin, got.Role)
	assert.True(t, got.LastLoginAt.Equal(updated.LastLoginAt))
}

func (suite *AuthRepositoryTestSuite) TestGetByID_NotFound() {
	t := suite.T()

	user, err := suite.users.GetByID(suite.ctx, 42)
	assert.Nil(t, user)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *AuthRepositoryTestSuite) TestSessionLifecycle() {
	t := suite.T()
	user, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "user@example.com",
			DisplayName: "User One",
			AvatarURL:   "",
		},
		domain.UserRoleUser,
		time.Now(),
	)
	assert.NoError(t, err)

	now := time.Now().UTC()
	created, err := suite.sessions.Create(
		suite.ctx,
		&domain.Session{
			TokenHash: "hash-1",
			UserID:    user.ID,
			ExpiresAt: now.Add(time.Hour),
			Metadata:  map[string]any{"ip": "127.0.0.1"},
		},
	)
	assert.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.NotEqual(t, now, created.CreatedAt)

	found, err := suite.sessions.GetActiveByTokenHash(suite.ctx, "hash-1", now)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, user.ID, found.UserID)
	assert.Equal(t, "127.0.0.1", found.Metadata["ip"])

	err = suite.sessions.RevokeByTokenHash(suite.ctx, "hash-1", now.Add(10*time.Minute))
	assert.NoError(t, err)

	found, err = suite.sessions.GetActiveByTokenHash(suite.ctx, "hash-1", now.Add(11*time.Minute))
	assert.Nil(t, found)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *AuthRepositoryTestSuite) TestGetActiveByTokenHash_ExpiredSession() {
	t := suite.T()
	user, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "user@example.com",
			DisplayName: "User One",
			AvatarURL:   "",
		},
		domain.UserRoleUser,
		time.Now(),
	)
	assert.NoError(t, err)

	now := time.Now().UTC()
	_, err = suite.sessions.Create(
		suite.ctx,
		&domain.Session{
			TokenHash: "expired-hash",
			UserID:    user.ID,
			ExpiresAt: now.Add(-time.Minute),
			Metadata:  map[string]any{},
		},
	)
	assert.NoError(t, err)

	session, err := suite.sessions.GetActiveByTokenHash(suite.ctx, "expired-hash", now)
	assert.Nil(t, session)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (suite *AuthRepositoryTestSuite) TestCleanupExpiredOrRevoked() {
	t := suite.T()
	user, err := suite.users.UpsertGoogleUser(
		suite.ctx,
		domain.GoogleIdentity{
			Subject:     "google-subject-1",
			Email:       "user@example.com",
			DisplayName: "User One",
			AvatarURL:   "",
		},
		domain.UserRoleUser,
		time.Now(),
	)
	assert.NoError(t, err)

	now := time.Now().UTC()
	revokedAt := now.Add(-5 * time.Minute)
	_, err = suite.sessions.Create(suite.ctx, &domain.Session{
		TokenHash: "active",
		UserID:    user.ID,
		ExpiresAt: now.Add(time.Hour),
		Metadata:  map[string]any{},
	})
	assert.NoError(t, err)
	_, err = suite.sessions.Create(suite.ctx, &domain.Session{
		TokenHash: "expired",
		UserID:    user.ID,
		ExpiresAt: now.Add(-time.Hour),
		Metadata:  map[string]any{},
	})
	assert.NoError(t, err)
	_, err = suite.sessions.Create(suite.ctx, &domain.Session{
		TokenHash: "revoked",
		UserID:    user.ID,
		ExpiresAt: now.Add(time.Hour),
		RevokedAt: &revokedAt,
		Metadata:  map[string]any{},
	})
	assert.NoError(t, err)

	deleted, err := suite.sessions.CleanupExpiredOrRevoked(suite.ctx, now)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	active, err := suite.sessions.GetActiveByTokenHash(suite.ctx, "active", now)
	assert.NoError(t, err)
	assert.NotNil(t, active)
}

func TestAuthRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(AuthRepositoryTestSuite))
}
