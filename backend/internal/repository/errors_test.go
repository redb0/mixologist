package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDBError(t *testing.T) {
	unknown := errors.New("something unexpected")

	cases := []struct {
		name   string
		err    error
		want   error
		wantIs error // если задан — проверяем errors.Is
	}{
		{
			name: "nil",
			err:  nil,
			want: nil,
		},
		{
			name:   "sql.ErrNoRows",
			err:    sql.ErrNoRows,
			wantIs: domain.ErrNotFound,
		},
		{
			name:   "unique violation 23505",
			err:    &pq.Error{Code: "23505", Detail: "Key (name)=(Джин) already exists."},
			wantIs: domain.ErrAlreadyExists,
		},
		{
			name:   "foreign key 23503",
			err:    &pq.Error{Code: "23503", Detail: "Key is not present"},
			wantIs: domain.ErrResourceInUse,
		},
		{
			name:   "check violation 23514",
			err:    &pq.Error{Code: "23514"},
			wantIs: domain.ErrInvalidIngredientData,
		},
		{
			name:   "deadlock 40P01",
			err:    &pq.Error{Code: "40P01"},
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "query canceled 57014",
			err:    &pq.Error{Code: "57014"},
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "timeout string",
			err:    errors.New("i/o timeout"),
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "deadline exceeded string",
			err:    errors.New("context deadline exceeded"),
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "context canceled string",
			err:    errors.New("context canceled"),
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "connection refused string",
			err:    errors.New("connection refused"),
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "context.Canceled",
			err:    context.Canceled,
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "context.DeadlineExceeded",
			err:    context.DeadlineExceeded,
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name:   "wrapped context.DeadlineExceeded",
			err:    fmt.Errorf("query: %w", context.DeadlineExceeded),
			wantIs: domain.ErrServiceUnavailable,
		},
		{
			name: "unknown error passthrough",
			err:  unknown,
			want: unknown,
		},
		{
			name: "unknown pq code passthrough",
			err:  &pq.Error{Code: "42P01", Message: "undefined_table"},
			want: &pq.Error{Code: "42P01", Message: "undefined_table"},
		},
		{
			name:   "wrapped unique violation",
			err:    fmt.Errorf("exec: %w", &pq.Error{Code: "23505"}),
			wantIs: domain.ErrAlreadyExists,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDBError(tt.err)

			if tt.want == nil && tt.wantIs == nil {
				assert.Nil(t, got)
				return
			}

			if tt.wantIs != nil {
				require.Error(t, got)
				assert.True(t, errors.Is(got, tt.wantIs), "got=%v wantIs=%v", got, tt.wantIs)
				// детали Postgres не должны попадать в Error()
				if errors.Is(tt.wantIs, domain.ErrAlreadyExists) {
					assert.Equal(t, "запись уже существует", got.Error())
				}
				if errors.Is(tt.wantIs, domain.ErrResourceInUse) {
					assert.Equal(t, resourceInUseMessage, got.Error())
				}
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
