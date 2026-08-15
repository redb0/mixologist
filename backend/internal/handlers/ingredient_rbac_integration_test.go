package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
	"github.com/redb0/mixologist/internal/middleware"
	"github.com/redb0/mixologist/internal/testutil"
)

type ingredientRouteCase struct {
	name        string
	method      string
	path        func(id uint) string
	body        []byte
	contentType string
	needsID     bool
	adminStatus int
}

func ingredientReadCases() []ingredientRouteCase {
	return []ingredientRouteCase{
		{
			name:        "list",
			method:      http.MethodGet,
			path:        func(uint) string { return "/api/v1/ingredients" },
			adminStatus: http.StatusOK,
		},
		{
			name:        "get",
			method:      http.MethodGet,
			path:        func(id uint) string { return "/api/v1/ingredients/" + strconv.Itoa(int(id)) },
			needsID:     true,
			adminStatus: http.StatusOK,
		},
		{
			name:   "get icon",
			method: http.MethodGet,
			path: func(id uint) string {
				return "/api/v1/ingredients/" + strconv.Itoa(int(id)) + "/icon"
			},
			needsID:     true,
			adminStatus: http.StatusOK,
		},
	}
}

func ingredientMutatingCases() []ingredientRouteCase {
	return []ingredientRouteCase{
		{
			name:   "create",
			method: http.MethodPost,
			path:   func(uint) string { return "/api/v1/ingredients" },
			body: []byte(`{
				"name": "RBAC Джин",
				"unit_measurement": "мл",
				"abv": "крепкий",
				"ingredient_type": "крепкая часть"
			}`),
			contentType: "application/json",
			adminStatus: http.StatusCreated,
		},
		{
			name:        "update",
			method:      http.MethodPatch,
			path:        func(id uint) string { return "/api/v1/ingredients/" + strconv.Itoa(int(id)) },
			body:        []byte(`{"version":1,"description":"RBAC update"}`),
			contentType: "application/json",
			needsID:     true,
			adminStatus: http.StatusOK,
		},
		{
			name:        "delete",
			method:      http.MethodDelete,
			path:        func(id uint) string { return "/api/v1/ingredients/" + strconv.Itoa(int(id)) },
			needsID:     true,
			adminStatus: http.StatusNoContent,
		},
		{
			name:   "put icon",
			method: http.MethodPut,
			path: func(id uint) string {
				return "/api/v1/ingredients/" + strconv.Itoa(int(id)) + "/icon"
			},
			body:        []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			contentType: "application/octet-stream",
			needsID:     true,
			adminStatus: http.StatusNoContent,
		},
	}
}

var rbacIngredientSeq atomic.Int64

func (suite *IngredientHandlerTestSuite) seedRBACIngredient() *domain.Ingredient {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "RBAC " + strconv.FormatInt(rbacIngredientSeq.Add(1), 10),
		Description:     "для RBAC",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Free,
		IngredientType:  domain.FreePart,
		Icon:            []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	})
	suite.Require().NoError(err)
	return created
}

func (suite *IngredientHandlerTestSuite) newIngredientRequest(tt ingredientRouteCase, id uint) *http.Request {
	var body *bytes.Reader
	if tt.body != nil {
		body = bytes.NewReader(tt.body)
	} else {
		body = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(tt.method, tt.path(id), body)
	if tt.contentType != "" {
		req.Header.Set("Content-Type", tt.contentType)
	}
	return req
}

func (suite *IngredientHandlerTestSuite) TestIngredientsRBAC_NoCookie() {
	ingredient := suite.seedRBACIngredient()

	for _, tt := range ingredientReadCases() {
		suite.Run("read "+tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = ingredient.ID
			}
			w := suite.serve(suite.newIngredientRequest(tt, id))
			suite.Equal(http.StatusOK, w.Code)
		})
	}

	for _, tt := range ingredientMutatingCases() {
		suite.Run("mutate "+tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = ingredient.ID
			}
			w := suite.serve(suite.newIngredientRequest(tt, id))
			suite.Equal(http.StatusUnauthorized, w.Code)
			suite.assertStructuredError(w, httperr.CodeUnauthorized, "требуется аутентификация")
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestIngredientsRBAC_UserRole() {
	ingredient := suite.seedRBACIngredient()

	for _, tt := range ingredientReadCases() {
		suite.Run("read "+tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = ingredient.ID
			}
			w := suite.serveWithToken(suite.newIngredientRequest(tt, id), suite.userToken)
			suite.Equal(http.StatusOK, w.Code)
		})
	}

	for _, tt := range ingredientMutatingCases() {
		suite.Run("mutate "+tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = suite.seedRBACIngredient().ID
			}
			w := suite.serveWithToken(suite.newIngredientRequest(tt, id), suite.userToken)
			suite.Equal(http.StatusForbidden, w.Code)
			suite.assertStructuredError(w, httperr.CodeForbidden, "недостаточно прав")
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestIngredientsRBAC_ExpiredAndRevokedSession() {
	ingredient := suite.seedRBACIngredient()
	now := time.Now().UTC()

	_, expiredToken, err := testutil.CreateUserSession(
		suite.ctx,
		suite.authService,
		domain.GoogleIdentity{
			Subject:     "expired-subject",
			Email:       "expired@example.com",
			DisplayName: "Expired",
		},
		now.Add(-2*time.Hour),
		time.Hour,
	)
	suite.Require().NoError(err)

	_, revokedToken, err := testutil.CreateUserSession(
		suite.ctx,
		suite.authService,
		domain.GoogleIdentity{
			Subject:     "revoked-subject",
			Email:       "revoked@example.com",
			DisplayName: "Revoked",
		},
		now,
		time.Hour,
	)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.authService.RevokeSessionByRawToken(suite.ctx, revokedToken, now))

	tokens := []struct {
		name  string
		token string
	}{
		{name: "expired", token: expiredToken},
		{name: "revoked", token: revokedToken},
	}

	for _, session := range tokens {
		for _, tt := range ingredientMutatingCases() {
			suite.Run(session.name+" "+tt.name, func() {
				id := uint(0)
				if tt.needsID {
					id = ingredient.ID
				}
				w := suite.serveWithToken(suite.newIngredientRequest(tt, id), session.token)
				suite.Equal(http.StatusUnauthorized, w.Code)
				suite.assertStructuredError(w, httperr.CodeUnauthorized, "сессия недействительна")
			})
		}
	}
}

func (suite *IngredientHandlerTestSuite) TestIngredientsRBAC_AdminCanMutate() {
	for _, tt := range ingredientMutatingCases() {
		suite.Run(tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = suite.seedRBACIngredient().ID
			}
			w := suite.serveWithToken(suite.newIngredientRequest(tt, id), suite.adminToken)
			suite.Equal(tt.adminStatus, w.Code)
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestIngredientsRBAC_AdminWithoutCSRF() {
	for _, tt := range ingredientMutatingCases() {
		suite.Run(tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = suite.seedRBACIngredient().ID
			}
			req := suite.newIngredientRequest(tt, id)
			req.AddCookie(&http.Cookie{
				Name:  suite.authConfig.SessionCookieName,
				Value: suite.adminToken,
			})
			w := suite.serve(req)
			suite.Equal(http.StatusForbidden, w.Code)
			suite.assertStructuredError(w, middleware.CodeCSRFTokenInvalid, "Некорректный CSRF token")
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestIngredientsRBAC_AdminWithInvalidCSRF() {
	for _, tt := range ingredientMutatingCases() {
		suite.Run(tt.name, func() {
			id := uint(0)
			if tt.needsID {
				id = suite.seedRBACIngredient().ID
			}
			req := suite.newIngredientRequest(tt, id)
			req.AddCookie(&http.Cookie{
				Name:  suite.authConfig.SessionCookieName,
				Value: suite.adminToken,
			})
			req.Header.Set(suite.authConfig.CSRFHeaderName, "invalid-csrf-token")

			w := suite.serve(req)
			suite.Equal(http.StatusForbidden, w.Code)
			suite.assertStructuredError(w, middleware.CodeCSRFTokenInvalid, "Некорректный CSRF token")
		})
	}
}
