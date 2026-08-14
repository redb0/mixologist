package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/repository"
	"github.com/redb0/mixologist/internal/router"
	"github.com/redb0/mixologist/internal/services"
	"github.com/redb0/mixologist/internal/testutil"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ContractTestSuite struct {
	suite.Suite
	ctx           context.Context
	pgContainer   *testutil.PostgresContainer
	router        *gin.Engine
	authService   services.AuthService
	authConfig    config.AuthConfig
	adminToken    string
	userToken     string
	doc           *openapi3.T
	openapiRouter routers.Router
}

type requestAuthMode int

const (
	authNone requestAuthMode = iota
	authAdmin
	authUser
	authAdminInvalidCSRF
)

type mutatingIngredientCase struct {
	name        string
	method      string
	path        func(uint) string
	body        []byte
	contentType string
	needsID     bool
}

func mutatingIngredientCases() []mutatingIngredientCase {
	return []mutatingIngredientCase{
		{
			name:        "create",
			method:      http.MethodPost,
			path:        func(uint) string { return "/api/v1/ingredients" },
			body:        createIngredientJSON("RBAC ingredient"),
			contentType: "application/json",
		},
		{
			name:        "update",
			method:      http.MethodPatch,
			path:        func(id uint) string { return "/api/v1/ingredients/" + itoa(id) },
			body:        []byte(`{"version":1,"description":"changed"}`),
			contentType: "application/json",
			needsID:     true,
		},
		{
			name:    "delete",
			method:  http.MethodDelete,
			path:    func(id uint) string { return "/api/v1/ingredients/" + itoa(id) },
			needsID: true,
		},
		{
			name:        "put icon",
			method:      http.MethodPut,
			path:        func(id uint) string { return "/api/v1/ingredients/" + itoa(id) + "/icon" },
			body:        []byte{0x89, 0x50, 0x4E, 0x47},
			contentType: "application/octet-stream",
			needsID:     true,
		},
	}
}

func (suite *ContractTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.ctx = context.Background()

	pgContainer, err := testutil.SetupContainerAndMigrations(suite.ctx)
	if err != nil {
		suite.T().Fatalf("Не удалось подготовить тестовую БД: %v", err)
	}
	suite.pgContainer = pgContainer

	repo := repository.NewIngredientRepository(suite.pgContainer.DB)
	suite.authConfig = testutil.TestAuthConfig()
	suite.authService = services.NewAuthService(
		repository.NewUserRepository(suite.pgContainer.DB),
		repository.NewSessionRepository(suite.pgContainer.DB),
		[]string{"admin@example.com"},
		nil,
		"google-client-id",
	)
	service := services.NewIngredientService(repo)
	ingredientController := handlers.NewIngredientController(service)
	healthController := handlers.NewHealthController(suite.pgContainer.DB)

	suite.router = router.New(router.Dependencies{
		HealthController:     healthController,
		IngredientController: ingredientController,
		AuthService:          suite.authService,
		AuthConfig:           suite.authConfig,
	})

	_, filename, _, _ := runtime.Caller(0)
	specPath := filepath.Join(filepath.Dir(filename), "../../../api/openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	require.NoError(suite.T(), err)
	err = doc.Validate(loader.Context)
	require.NoError(suite.T(), err)
	suite.doc = doc
	openapiRouter, err := gorillamux.NewRouter(doc)
	require.NoError(suite.T(), err)
	suite.openapiRouter = openapiRouter
}

func (suite *ContractTestSuite) SetupTest() {
	now := time.Now().UTC()
	_, adminToken, err := testutil.CreateUserSession(
		suite.ctx,
		suite.authService,
		domain.GoogleIdentity{
			Subject:     "admin-subject",
			Email:       "admin@example.com",
			DisplayName: "Admin",
		},
		now,
		time.Hour,
	)
	suite.Require().NoError(err)
	suite.adminToken = adminToken

	_, userToken, err := testutil.CreateUserSession(
		suite.ctx,
		suite.authService,
		domain.GoogleIdentity{
			Subject:     "user-subject",
			Email:       "user@example.com",
			DisplayName: "User",
		},
		now,
		time.Hour,
	)
	suite.Require().NoError(err)
	suite.userToken = userToken
}

func (suite *ContractTestSuite) TearDownTest() {
	if err := testutil.TruncateIngredients(suite.pgContainer.DB); err != nil {
		suite.T().Fatalf("Не удалось очистить таблицу ingredients: %v", err)
	}
	if err := testutil.TruncateAuthTables(suite.pgContainer.DB); err != nil {
		suite.T().Fatalf("Не удалось очистить auth таблицы: %v", err)
	}
}

func (suite *ContractTestSuite) TearDownSuite() {
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

func (suite *ContractTestSuite) validate(method, target string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	return suite.validateAs(authAdmin, method, target, body, contentType)
}

func (suite *ContractTestSuite) validateAs(
	authMode requestAuthMode,
	method string,
	target string,
	body io.Reader,
	contentType string,
) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	switch authMode {
	case authAdmin:
		testutil.AttachSession(req, suite.authConfig, suite.adminToken)
	case authUser:
		testutil.AttachSession(req, suite.authConfig, suite.userToken)
	case authAdminInvalidCSRF:
		testutil.AttachSession(req, suite.authConfig, suite.adminToken)
		req.Header.Set(suite.authConfig.CSRFHeaderName, "invalid-csrf-token")
	}
	suite.router.ServeHTTP(w, req)

	route, pathParams, err := suite.openapiRouter.FindRoute(req)
	require.NoError(suite.T(), err)

	resp := w.Result()
	resp.Request = req

	err = openapi3filter.ValidateResponse(
		suite.ctx,
		&openapi3filter.ResponseValidationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
			},
			Status: w.Code,
			Header: resp.Header,
			Body:   io.NopCloser(bytes.NewReader(w.Body.Bytes())),
		},
	)
	require.NoError(suite.T(), err)

	require.NotEmpty(suite.T(), w.Header().Get("X-Request-ID"))
	return w
}

func (suite *ContractTestSuite) TestHealth_200() {
	suite.validateAs(authNone, http.MethodGet, "/health", nil, "")
}

func (suite *ContractTestSuite) TestListIngredients_200_Empty() {
	suite.validateAs(authNone, http.MethodGet, "/api/v1/ingredients", nil, "")
}

func (suite *ContractTestSuite) TestCreateAndGetIngredient() {
	createBody := `{
		"name": "Джин",
		"description": "London dry gin",
		"unit_measurement": "мл",
		"abv": "крепкий",
		"ingredient_type": "крепкая часть"
	}`
	createW := suite.validate(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(createBody), "application/json")
	require.Equal(suite.T(), http.StatusCreated, createW.Code)

	var created handlers.IngredientResponse
	require.NoError(suite.T(), json.Unmarshal(createW.Body.Bytes(), &created))

	suite.validateAs(authNone, http.MethodGet, "/api/v1/ingredients/"+itoa(created.ID), nil, "")
}

func (suite *ContractTestSuite) TestListIngredients_400() {
	suite.validateAs(authNone, http.MethodGet, "/api/v1/ingredients?pageSize=abc", nil, "")
}

func (suite *ContractTestSuite) TestGetIngredient_404() {
	suite.validateAs(authNone, http.MethodGet, "/api/v1/ingredients/42", nil, "")
}

func (suite *ContractTestSuite) TestUpdateIngredient_200() {
	createBody := `{
		"name": "Ром",
		"unit_measurement": "мл",
		"abv": "крепкий",
		"ingredient_type": "крепкая часть"
	}`
	createW := suite.validate(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(createBody), "application/json")
	var created handlers.IngredientResponse
	require.NoError(suite.T(), json.Unmarshal(createW.Body.Bytes(), &created))

	patchBody := `{"version":1,"description":"Белый ром"}`
	suite.validate(
		http.MethodPatch,
		"/api/v1/ingredients/"+itoa(created.ID),
		bytes.NewBufferString(patchBody),
		"application/json",
	)
}

func (suite *ContractTestSuite) TestDeleteIngredient_204() {
	createBody := `{
		"name": "Тоник",
		"unit_measurement": "мл",
		"abv": "безалкогольный",
		"ingredient_type": "безалкогольная часть"
	}`
	createW := suite.validate(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(createBody), "application/json")
	var created handlers.IngredientResponse
	require.NoError(suite.T(), json.Unmarshal(createW.Body.Bytes(), &created))

	suite.validate(http.MethodDelete, "/api/v1/ingredients/"+itoa(created.ID), nil, "")
}

func (suite *ContractTestSuite) TestMutatingIngredients_401_WithoutAuth() {
	suite.assertMutatingIngredientStatus(authNone, http.StatusUnauthorized, "seed401", true)
}

func (suite *ContractTestSuite) TestMutatingIngredients_403_UserRole() {
	suite.assertMutatingIngredientStatus(authUser, http.StatusForbidden, "seed403user", true)
}

func (suite *ContractTestSuite) TestMutatingIngredients_403_InvalidCSRF() {
	suite.assertMutatingIngredientStatus(authAdminInvalidCSRF, http.StatusForbidden, "seed403csrf", false)
}

func (suite *ContractTestSuite) createIngredientAsAdmin(name string) uint {
	createW := suite.validateAs(
		authAdmin,
		http.MethodPost,
		"/api/v1/ingredients",
		bytes.NewReader(createIngredientJSON(name)),
		"application/json",
	)
	require.Equal(suite.T(), http.StatusCreated, createW.Code)

	var created handlers.IngredientResponse
	require.NoError(suite.T(), json.Unmarshal(createW.Body.Bytes(), &created))
	return created.ID
}

func (suite *ContractTestSuite) assertMutatingIngredientStatus(
	authMode requestAuthMode,
	wantStatus int,
	seedPrefix string,
	reseedPerCase bool,
) {
	sharedID := uint(0)
	if !reseedPerCase {
		sharedID = suite.createIngredientAsAdmin(seedPrefix + "-shared")
	}

	for _, tc := range mutatingIngredientCases() {
		suite.Run(tc.name, func() {
			targetID := uint(0)
			if tc.needsID {
				if reseedPerCase {
					targetID = suite.createIngredientAsAdmin(seedPrefix + "-" + tc.name)
				} else {
					targetID = sharedID
				}
			}
			w := suite.validateAs(
				authMode,
				tc.method,
				tc.path(targetID),
				requestBodyReader(tc.body),
				tc.contentType,
			)
			require.Equal(suite.T(), wantStatus, w.Code)
		})
	}
}

func requestBodyReader(payload []byte) io.Reader {
	if len(payload) == 0 {
		return nil
	}
	return bytes.NewReader(payload)
}

func createIngredientJSON(name string) []byte {
	return []byte(fmt.Sprintf(
		`{"name":"%s","unit_measurement":"мл","abv":"крепкий","ingredient_type":"крепкая часть"}`,
		name,
	))
}

func itoa(id uint) string {
	return strconv.Itoa(int(id))
}

func TestContractTestSuite(t *testing.T) {
	suite.Run(t, new(ContractTestSuite))
}
