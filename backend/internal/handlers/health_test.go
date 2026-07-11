package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type stubPinger struct {
	err error
}

func (p stubPinger) PingContext(context.Context) error {
	return p.err
}

func TestHealthController_GetHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "healthy",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "database unavailable",
			pingErr:    errors.New("connection refused"),
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"status":"unavailable"}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			controller := NewHealthController(stubPinger{err: tt.pingErr})
			router.GET("/health", controller.GetHealth)

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

			assert.Equal(t, tt.wantStatus, response.Code)
			assert.JSONEq(t, tt.wantBody, response.Body.String())
		})
	}
}
