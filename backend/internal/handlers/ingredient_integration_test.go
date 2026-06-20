package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/repository"
	"github.com/redb0/mixologist/internal/services"
	"github.com/redb0/mixologist/internal/testutil"
	"github.com/stretchr/testify/suite"
)

type IngredientHandlerTestSuite struct {
	suite.Suite
	pgContainer *testutil.PostgresContainer
	repository  repository.IngredientRepository
	router      *gin.Engine
	ctx         context.Context
}

func (suite *IngredientHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.ctx = context.Background()
	pgContainer, err := testutil.SetupContainerAndMigrations(suite.ctx)
	if err != nil {
		suite.T().Fatalf("Не удалось подготовить тестовую БД: %v", err)
	}
	suite.pgContainer = pgContainer
	suite.repository = repository.NewIngredientRepository(suite.pgContainer.DB)

	service := services.NewIngredientService(suite.repository)
	controller := NewIngredientController(service)

	suite.router = gin.New()
	suite.router.GET("/ingredients/:id", controller.GetIngredient)
}

func (suite *IngredientHandlerTestSuite) TearDownTest() {
	err := testutil.TruncateIngredients(suite.pgContainer.DB)
	if err != nil {
		suite.T().Fatalf("Не удалось очистить таблицу ingredients: %v", err)
	}
}

func (suite *IngredientHandlerTestSuite) TearDownSuite() {
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

func (suite *IngredientHandlerTestSuite) TestGetIngredient_400() {
	cases := []struct {
		name string
		path string
	}{
		{name: "not a number", path: "/ingredients/not-a-number"},
		{name: "zero", path: "/ingredients/0"},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)

			var response gin.H
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
			suite.Equal("неверный ID ингредиента", response["error"])
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestGetIngredient_404() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ingredients/42", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal("Ингредиент не найден", response["error"])
}

func (suite *IngredientHandlerTestSuite) TestGetIngredient_200() {
	cases := []struct {
		name       string
		ingredient domain.Ingredient
	}{
		{name: "with icon", ingredient: domain.Ingredient{
			Name:            "Ром",
			Description:     "Белый ром",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Strong,
			IngredientType:  domain.StrongPart,
			Icon:            []byte{1, 2, 3},
		}},
		{name: "without icon", ingredient: domain.Ingredient{
			Name:            "Апельсиновый сок",
			Description:     "Свежевыжатый апельсиновый сок",
			UnitMeasurement: domain.UnitMl,
			ABV:             domain.Free,
			IngredientType:  domain.FreePart,
		}},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			createdIngredient, err := suite.repository.Create(suite.ctx, &tt.ingredient)
			suite.Require().NoError(err)

			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/ingredients/"+strconv.Itoa(int(createdIngredient.ID)), nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusOK, w.Code)

			var response IngredientResponse
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
			suite.Equal(createdIngredient.ID, response.ID)
			suite.Equal(createdIngredient.Name, response.Name)
			suite.Equal(createdIngredient.Description, response.Description)
			suite.Equal(createdIngredient.UnitMeasurement, response.UnitMeasurement)
			suite.Equal(createdIngredient.ABV, response.ABV)
			suite.Equal(createdIngredient.IngredientType, response.IngredientType)
			suite.True(createdIngredient.CreatedAt.Equal(response.CreatedAt))
			suite.Equal(len(createdIngredient.Icon) > 0, response.HasIcon)
		})
	}
}

func TestIngredientHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(IngredientHandlerTestSuite))
}
