package domain

import "time"

type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

func (r UserRole) IsValid() bool {
	switch r {
	case UserRoleUser, UserRoleAdmin:
		return true
	default:
		return false
	}
}

type GoogleIdentity struct {
	Subject     string
	Email       string
	DisplayName string
	AvatarURL   string
}

type User struct {
	ID            uint      `db:"id"`
	GoogleSubject string    `db:"google_subject"`
	Email         string    `db:"email"`
	DisplayName   string    `db:"display_name"`
	AvatarURL     string    `db:"avatar_url"`
	Role          UserRole  `db:"role"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	LastLoginAt   time.Time `db:"last_login_at"`
}

type Session struct {
	ID        uint           `db:"id"`
	TokenHash string         `db:"token_hash"`
	UserID    uint           `db:"user_id"`
	ExpiresAt time.Time      `db:"expires_at"`
	RevokedAt *time.Time     `db:"revoked_at"`
	Metadata  map[string]any `db:"metadata"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
}
