package domain_test

import (
	"testing"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestUserRole_IsValid(t *testing.T) {
	tests := []struct {
		name string
		role domain.UserRole
		want bool
	}{
		{name: "user", role: domain.UserRoleUser, want: true},
		{name: "admin", role: domain.UserRoleAdmin, want: true},
		{name: "empty", role: "", want: false},
		{name: "unknown", role: domain.UserRole("moderator"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.role.IsValid())
		})
	}
}
