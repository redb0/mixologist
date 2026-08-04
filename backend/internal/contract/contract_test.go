package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
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
	doc           *openapi3.T
	openapiRouter routers.Router
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
	service := services.NewIngredientService(repo)
	ingredientController := handlers.NewIngredientController(service)
	healthController := handlers.NewHealthController(suite.pgContainer.DB)

	suite.router = router.New(router.Dependencies{
		HealthController:     healthController,
		IngredientController: ingredientController,
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

func (suite *ContractTestSuite) TearDownTest() {
	err := testutil.TruncateIngredients(suite.pgContainer.DB)
	if err != nil {
		suite.T().Fatalf("Не удалось очистить таблицу ingredients: %v", err)
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
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
	suite.validate(http.MethodGet, "/health", nil, "")
}

func (suite *ContractTestSuite) TestListIngredients_200_Empty() {
	suite.validate(http.MethodGet, "/api/v1/ingredients", nil, "")
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

	suite.validate(http.MethodGet, "/api/v1/ingredients/"+itoa(created.ID), nil, "")
}

func (suite *ContractTestSuite) TestListIngredients_400() {
	suite.validate(http.MethodGet, "/api/v1/ingredients?pageSize=abc", nil, "")
}

func (suite *ContractTestSuite) TestGetIngredient_404() {
	suite.validate(http.MethodGet, "/api/v1/ingredients/42", nil, "")
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

func itoa(id uint) string {
	return strconv.Itoa(int(id))
}

func TestContractTestSuite(t *testing.T) {
	suite.Run(t, new(ContractTestSuite))
}
