package models

import "time"

type User struct {
	ID            uint      `db:"id"`
	GoogleSubject string    `db:"google_subject"`
	Email         string    `db:"email"`
	DisplayName   string    `db:"display_name"`
	AvatarURL     string    `db:"avatar_url"`
	Role          string    `db:"role"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	LastLoginAt   time.Time `db:"last_login_at"`
}

type Session struct {
	ID        uint       `db:"id"`
	TokenHash string     `db:"token_hash"`
	UserID    uint       `db:"user_id"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
	Metadata  []byte     `db:"metadata"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}
