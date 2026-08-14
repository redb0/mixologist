package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redb0/mixologist/internal/domain"
)

type mockUserRepository struct {
	upsertGoogleUser func(
		ctx context.Context,
		identity domain.GoogleIdentity,
		role domain.UserRole,
		loginAt time.Time,
	) (*domain.User, error)
	getByID func(ctx context.Context, id uint) (*domain.User, error)
}

func (m *mockUserRepository) UpsertGoogleUser(
	ctx context.Context,
	identity domain.GoogleIdentity,
	role domain.UserRole,
	loginAt time.Time,
) (*domain.User, error) {
	if m.upsertGoogleUser == nil {
		panic("unexpected call to UpsertGoogleUser")
	}
	return m.upsertGoogleUser(ctx, identity, role, loginAt)
}

func (m *mockUserRepository) GetByID(ctx context.Context, id uint) (*domain.User, error) {
	if m.getByID == nil {
		panic("unexpected call to GetByID")
	}
	return m.getByID(ctx, id)
}

type mockSessionRepository struct {
	create               func(ctx context.Context, session *domain.Session) (*domain.Session, error)
	getActiveByTokenHash func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error)
	revokeByTokenHash    func(ctx context.Context, tokenHash string, revokedAt time.Time) error
	cleanup              func(ctx context.Context, now time.Time) (int64, error)
}

func (m *mockSessionRepository) Create(ctx context.Context, session *domain.Session) (*domain.Session, error) {
	if m.create == nil {
		panic("unexpected call to Create")
	}
	return m.create(ctx, session)
}

func (m *mockSessionRepository) GetActiveByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*domain.Session, error) {
	if m.getActiveByTokenHash == nil {
		panic("unexpected call to GetActiveByTokenHash")
	}
	return m.getActiveByTokenHash(ctx, tokenHash, now)
}

func (m *mockSessionRepository) RevokeByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if m.revokeByTokenHash == nil {
		panic("unexpected call to RevokeByTokenHash")
	}
	return m.revokeByTokenHash(ctx, tokenHash, revokedAt)
}

func (m *mockSessionRepository) CleanupExpiredOrRevoked(ctx context.Context, now time.Time) (int64, error) {
	if m.cleanup == nil {
		panic("unexpected call to CleanupExpiredOrRevoked")
	}
	return m.cleanup(ctx, now)
}

func TestAuthService_UpsertGoogleUser_SetsAdminRoleByAllowlist(t *testing.T) {
	var gotIdentity domain.GoogleIdentity
	var gotRole domain.UserRole

	users := &mockUserRepository{
		upsertGoogleUser: func(
			ctx context.Context,
			identity domain.GoogleIdentity,
			role domain.UserRole,
			loginAt time.Time,
		) (*domain.User, error) {
			gotIdentity = identity
			gotRole = role
			return &domain.User{ID: 1, Role: role, Email: identity.Email}, nil
		},
	}
	service := newTestAuthService(users, &mockSessionRepository{}, []string{"admin@example.com"}, nil)

	user, err := service.UpsertGoogleUser(context.Background(), domain.GoogleIdentity{
		Subject:     "google-subject",
		Email:       " Admin@Example.com ",
		DisplayName: "Admin User",
		AvatarURL:   "https://img.example.com/avatar.png",
	}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotRole != domain.UserRoleAdmin {
		t.Fatalf("role mismatch: got %q want %q", gotRole, domain.UserRoleAdmin)
	}
	if gotIdentity.Email != "admin@example.com" {
		t.Fatalf("normalized email mismatch: got %q", gotIdentity.Email)
	}
	if user.Role != domain.UserRoleAdmin {
		t.Fatalf("user role mismatch: got %q", user.Role)
	}
}

func TestAuthService_CreateSession_StoresOnlyTokenHash(t *testing.T) {
	var savedSession *domain.Session
	repo := &mockSessionRepository{
		create: func(ctx context.Context, session *domain.Session) (*domain.Session, error) {
			savedCopy := *session
			savedSession = &savedCopy
			session.ID = 42
			return session, nil
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	rawToken, session, err := service.CreateSession(
		context.Background(),
		11,
		2*time.Hour,
		time.Now(),
		map[string]any{"ip": "127.0.0.1"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rawToken == "" {
		t.Fatal("raw token must not be empty")
	}
	if savedSession == nil {
		t.Fatal("session was not saved")
	}
	if savedSession.TokenHash == rawToken {
		t.Fatal("raw token must not be persisted")
	}
	if savedSession.TokenHash != hashToken(rawToken) {
		t.Fatal("token hash mismatch")
	}
	if session.ID == 0 {
		t.Fatal("session ID must be set by repository")
	}
}

func TestAuthService_CreateSession_RepositoryError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	repo := &mockSessionRepository{
		create: func(ctx context.Context, session *domain.Session) (*domain.Session, error) {
			return nil, repoErr
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	rawToken, session, err := service.CreateSession(context.Background(), 11, time.Hour, time.Now(), nil)
	if rawToken != "" || session != nil {
		t.Fatal("session must not be created on repository error")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestAuthService_GetActiveSessionByRawToken_RepositoryError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	repo := &mockSessionRepository{
		getActiveByTokenHash: func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error) {
			return nil, repoErr
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	session, err := service.GetActiveSessionByRawToken(context.Background(), "token", time.Now())
	if session != nil {
		t.Fatal("session should be nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestAuthService_GetActiveSessionByRawToken_NotFoundReturnsUnauthorized(t *testing.T) {
	repo := &mockSessionRepository{
		getActiveByTokenHash: func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error) {
			return nil, domain.NewErrNotFound("сессия не найдена")
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	session, err := service.GetActiveSessionByRawToken(context.Background(), "token", time.Now())
	if session != nil {
		t.Fatal("session should be nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthService_RevokeSessionByRawToken_Success(t *testing.T) {
	var gotHash string
	repo := &mockSessionRepository{
		revokeByTokenHash: func(ctx context.Context, tokenHash string, revokedAt time.Time) error {
			gotHash = tokenHash
			return nil
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	err := service.RevokeSessionByRawToken(context.Background(), "token", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHash != hashToken("token") {
		t.Fatal("token hash mismatch")
	}
}

func TestAuthService_RevokeSessionByRawToken_EmptyToken(t *testing.T) {
	service := newTestAuthService(&mockUserRepository{}, &mockSessionRepository{}, nil, nil)

	err := service.RevokeSessionByRawToken(context.Background(), "   ", time.Now())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthService_RevokeSessionByRawToken_RepositoryError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	repo := &mockSessionRepository{
		revokeByTokenHash: func(ctx context.Context, tokenHash string, revokedAt time.Time) error {
			return repoErr
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	err := service.RevokeSessionByRawToken(context.Background(), "token", time.Now())
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestAuthService_RevokeSessionByRawToken_NotFoundReturnsUnauthorized(t *testing.T) {
	repo := &mockSessionRepository{
		revokeByTokenHash: func(ctx context.Context, tokenHash string, revokedAt time.Time) error {
			return domain.NewErrNotFound("сессия не найдена")
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	err := service.RevokeSessionByRawToken(context.Background(), "token", time.Now())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthService_UpsertGoogleUser_AssignsUserRoleOutsideAllowlist(t *testing.T) {
	var gotRole domain.UserRole
	users := &mockUserRepository{
		upsertGoogleUser: func(
			ctx context.Context,
			identity domain.GoogleIdentity,
			role domain.UserRole,
			loginAt time.Time,
		) (*domain.User, error) {
			gotRole = role
			return &domain.User{ID: 1, Role: role, Email: identity.Email}, nil
		},
	}
	service := newTestAuthService(users, &mockSessionRepository{}, []string{"admin@example.com"}, nil)

	user, err := service.UpsertGoogleUser(context.Background(), domain.GoogleIdentity{
		Subject:     "google-subject",
		Email:       "user@example.com",
		DisplayName: "Regular User",
	}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRole != domain.UserRoleUser {
		t.Fatalf("role mismatch: got %q want %q", gotRole, domain.UserRoleUser)
	}
	if user.Role != domain.UserRoleUser {
		t.Fatalf("user role mismatch: got %q", user.Role)
	}
}

func TestAuthService_UpsertGoogleUser_Validation(t *testing.T) {
	service := newTestAuthService(&mockUserRepository{}, &mockSessionRepository{}, nil, nil)
	tests := []struct {
		name     string
		identity domain.GoogleIdentity
	}{
		{
			name:     "empty subject",
			identity: domain.GoogleIdentity{Email: "user@example.com", DisplayName: "User"},
		},
		{
			name:     "empty email",
			identity: domain.GoogleIdentity{Subject: "subject", DisplayName: "User"},
		},
		{
			name:     "whitespace email",
			identity: domain.GoogleIdentity{Subject: "subject", Email: "   ", DisplayName: "User"},
		},
		{
			name:     "empty display name",
			identity: domain.GoogleIdentity{Subject: "subject", Email: "user@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := service.UpsertGoogleUser(context.Background(), tt.identity, time.Now())
			if user != nil {
				t.Fatal("user should be nil")
			}
			if !errors.Is(err, domain.ErrInvalidAuthData) {
				t.Fatalf("expected invalid auth data, got %v", err)
			}
		})
	}
}

func TestAuthService_CreateSession_Validation(t *testing.T) {
	service := newTestAuthService(&mockUserRepository{}, &mockSessionRepository{}, nil, nil)

	_, session, err := service.CreateSession(context.Background(), 0, time.Hour, time.Now(), nil)
	if session != nil {
		t.Fatal("session should be nil")
	}
	if !errors.Is(err, domain.ErrInvalidAuthData) {
		t.Fatalf("expected invalid auth data for user_id=0, got %v", err)
	}

	_, session, err = service.CreateSession(context.Background(), 1, 0, time.Now(), nil)
	if session != nil {
		t.Fatal("session should be nil")
	}
	if !errors.Is(err, domain.ErrInvalidAuthData) {
		t.Fatalf("expected invalid auth data for ttl=0, got %v", err)
	}
}

func TestAuthService_GetActiveSessionByRawToken_EmptyToken(t *testing.T) {
	service := newTestAuthService(&mockUserRepository{}, &mockSessionRepository{}, nil, nil)

	session, err := service.GetActiveSessionByRawToken(context.Background(), "   ", time.Now())
	if session != nil {
		t.Fatal("session should be nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthService_GetUserBySessionToken_UserNotFound(t *testing.T) {
	repo := &mockSessionRepository{
		getActiveByTokenHash: func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error) {
			return &domain.Session{UserID: 7}, nil
		},
	}
	users := &mockUserRepository{
		getByID: func(ctx context.Context, id uint) (*domain.User, error) {
			return nil, domain.NewErrNotFound("пользователь не найден")
		},
	}
	service := newTestAuthService(users, repo, nil, nil)

	user, err := service.GetUserBySessionToken(context.Background(), "token", time.Now())
	if user != nil {
		t.Fatal("user should be nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthService_CleanupExpiredOrRevokedSessions(t *testing.T) {
	repo := &mockSessionRepository{
		cleanup: func(ctx context.Context, now time.Time) (int64, error) {
			return 3, nil
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	deleted, err := service.CleanupExpiredOrRevokedSessions(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted count mismatch: got %d", deleted)
	}
}

func TestAuthService_GetUserBySessionToken_UserLookupError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	repo := &mockSessionRepository{
		getActiveByTokenHash: func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error) {
			return &domain.Session{UserID: 7}, nil
		},
	}
	users := &mockUserRepository{
		getByID: func(ctx context.Context, id uint) (*domain.User, error) {
			return nil, repoErr
		},
	}
	service := newTestAuthService(users, repo, nil, nil)

	user, err := service.GetUserBySessionToken(context.Background(), "token", time.Now())
	if user != nil {
		t.Fatal("user should be nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestAuthService_GetUserBySessionToken_InvalidSession(t *testing.T) {
	repo := &mockSessionRepository{
		getActiveByTokenHash: func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error) {
			return nil, domain.NewErrNotFound("сессия не найдена")
		},
	}
	service := newTestAuthService(&mockUserRepository{}, repo, nil, nil)

	user, err := service.GetUserBySessionToken(context.Background(), "token", time.Now())
	if user != nil {
		t.Fatal("user should be nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthService_GetUserBySessionToken(t *testing.T) {
	repo := &mockSessionRepository{
		getActiveByTokenHash: func(ctx context.Context, tokenHash string, now time.Time) (*domain.Session, error) {
			return &domain.Session{UserID: 7}, nil
		},
	}
	users := &mockUserRepository{
		getByID: func(ctx context.Context, id uint) (*domain.User, error) {
			if id != 7 {
				t.Fatalf("unexpected user id: %d", id)
			}
			return &domain.User{ID: 7, Email: "user@example.com"}, nil
		},
	}
	service := newTestAuthService(users, repo, nil, nil)

	user, err := service.GetUserBySessionToken(context.Background(), "token", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 7 {
		t.Fatalf("user ID mismatch: got %d", user.ID)
	}
}
