package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/middleware"
	"github.com/redb0/mixologist/internal/repository"
	"github.com/redb0/mixologist/internal/router"
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
	controller := handlers.NewIngredientController(service)
	healthController := handlers.NewHealthController(suite.pgContainer.DB)

	suite.router = router.New(router.Dependencies{
		HealthController:     healthController,
		IngredientController: controller,
	})
}

func (suite *IngredientHandlerTestSuite) assertStructuredError(
	w *httptest.ResponseRecorder,
	wantCode string,
	wantMsg string,
) {
	var response handlers.ErrorResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal(wantCode, response.Error.Code)
	suite.Equal(wantMsg, response.Error.Message)
	suite.NotEmpty(response.Error.RequestID)
	suite.Equal(response.Error.RequestID, w.Header().Get(middleware.RequestIDHeader))
}

func (suite *IngredientHandlerTestSuite) assertStructuredErrorCode(
	w *httptest.ResponseRecorder,
	wantCode string,
) {
	var response handlers.ErrorResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal(wantCode, response.Error.Code)
	suite.NotEmpty(response.Error.Message)
	suite.NotEmpty(response.Error.RequestID)
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

func (suite *IngredientHandlerTestSuite) TestListIngredients_200_Empty() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response handlers.IngredientListResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Empty(response.Ingredients)
	suite.Empty(response.NextPageToken)
	suite.Equal(0, response.TotalSize)
}

func (suite *IngredientHandlerTestSuite) TestListIngredients_200() {
	withIcon, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "С иконкой",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
		Icon:            []byte{1, 2, 3},
	})
	suite.Require().NoError(err)

	withoutIcon, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Ром",
		Description:     "Без иконки",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response handlers.IngredientListResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Require().Len(response.Ingredients, 2)
	suite.Equal(2, response.TotalSize)
	suite.Empty(response.NextPageToken)

	// ORDER BY created_at DESC — последний созданный первым
	suite.Equal(withoutIcon.ID, response.Ingredients[0].ID)
	suite.Equal("Ром", response.Ingredients[0].Name)
	suite.Equal("Без иконки", response.Ingredients[0].Description)
	suite.Equal(domain.UnitMl, response.Ingredients[0].UnitMeasurement)
	suite.Equal(domain.Strong, response.Ingredients[0].ABV)
	suite.Equal(domain.StrongPart, response.Ingredients[0].IngredientType)
	suite.False(response.Ingredients[0].HasIcon)
	suite.False(response.Ingredients[0].CreatedAt.IsZero())

	suite.Equal(withIcon.ID, response.Ingredients[1].ID)
	suite.Equal("Джин", response.Ingredients[1].Name)
	suite.Equal("С иконкой", response.Ingredients[1].Description)
	suite.True(response.Ingredients[1].HasIcon)
	suite.False(response.Ingredients[1].CreatedAt.IsZero())
}

func (suite *IngredientHandlerTestSuite) TestListIngredients_200_PaginationAndFilter() {
	_, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "Первый",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)
	_, err = suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Ром",
		Description:     "Второй",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)
	_, err = suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Тоник",
		Description:     "Третий",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Free,
		IngredientType:  domain.FreePart,
	})
	suite.Require().NoError(err)

	firstPageW := httptest.NewRecorder()
	firstPageReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients?pageSize=2&sort=created_at&order=desc", nil)
	suite.router.ServeHTTP(firstPageW, firstPageReq)

	suite.Equal(http.StatusOK, firstPageW.Code)
	var firstPage handlers.IngredientListResponse
	suite.Require().NoError(json.Unmarshal(firstPageW.Body.Bytes(), &firstPage))
	suite.Len(firstPage.Ingredients, 2)
	suite.Equal(3, firstPage.TotalSize)
	suite.NotEmpty(firstPage.NextPageToken)

	secondPageW := httptest.NewRecorder()
	secondPageReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/ingredients?pageSize=2&sort=created_at&order=desc&pageToken="+firstPage.NextPageToken,
		nil,
	)
	suite.router.ServeHTTP(secondPageW, secondPageReq)

	suite.Equal(http.StatusOK, secondPageW.Code)
	var secondPage handlers.IngredientListResponse
	suite.Require().NoError(json.Unmarshal(secondPageW.Body.Bytes(), &secondPage))
	suite.Len(secondPage.Ingredients, 1)
	suite.Equal(3, secondPage.TotalSize)
	suite.Empty(secondPage.NextPageToken)

	filterW := httptest.NewRecorder()
	filterParams := url.Values{}
	filterParams.Set("name", "ник")
	filterParams.Set("abv", "безалкогольный")
	filterParams.Set("ingredient_type", "безалкогольная часть")
	filterReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients?"+filterParams.Encode(), nil)
	suite.router.ServeHTTP(filterW, filterReq)

	suite.Equal(http.StatusOK, filterW.Code)
	var filtered handlers.IngredientListResponse
	suite.Require().NoError(json.Unmarshal(filterW.Body.Bytes(), &filtered))
	suite.Len(filtered.Ingredients, 1)
	suite.Equal("Тоник", filtered.Ingredients[0].Name)
}

func (suite *IngredientHandlerTestSuite) TestListIngredients_400_InvalidQueryParams() {
	cases := []struct {
		name       string
		path       string
		wantErrMsg string
	}{
		{name: "pageSize not number", path: "/api/v1/ingredients?pageSize=abc", wantErrMsg: "недопустимое значение pageSize"},
		{name: "pageSize zero", path: "/api/v1/ingredients?pageSize=0", wantErrMsg: "pageSize должен быть от 1 до 100"},
		{name: "pageSize too large", path: "/api/v1/ingredients?pageSize=101", wantErrMsg: "pageSize должен быть от 1 до 100"},
		{name: "invalid sort", path: "/api/v1/ingredients?sort=invalid", wantErrMsg: "недопустимое значение sort"},
		{name: "invalid order", path: "/api/v1/ingredients?order=invalid", wantErrMsg: "недопустимое значение order"},
		{name: "invalid abv", path: "/api/v1/ingredients?abv=invalid", wantErrMsg: "недопустимое значение abv"},
		{
			name:       "invalid ingredient_type",
			path:       "/api/v1/ingredients?ingredient_type=invalid",
			wantErrMsg: "недопустимое значение ingredient_type",
		},
		{
			name:       "invalid page token",
			path:       "/api/v1/ingredients?pageToken=not-a-token",
			wantErrMsg: "Некорректный или несовместимый pageToken",
		},
	}

	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)
			if tt.name == "invalid page token" {
				suite.assertStructuredError(w, handlers.CodeInvalidPageToken, tt.wantErrMsg)
			} else {
				suite.assertStructuredError(w, handlers.CodeValidationError, tt.wantErrMsg)
			}
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestGetIngredientIcon_200() {
	icon := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	ingredient, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
		Icon:            icon,
	})
	suite.Require().NoError(err)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/"+strconv.Itoa(int(ingredient.ID))+"/icon", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)
	suite.Equal("image/png", w.Header().Get("Content-Type"))
	suite.Equal(strconv.Itoa(len(icon)), w.Header().Get("Content-Length"))
	suite.Equal("public, max-age=300, must-revalidate", w.Header().Get("Cache-Control"))
	suite.Equal(icon, w.Body.Bytes())
}

func (suite *IngredientHandlerTestSuite) TestGetIngredientIcon_404_Empty() {
	ingredient, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Тоник",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Free,
		IngredientType:  domain.FreePart,
	})
	suite.Require().NoError(err)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/"+strconv.Itoa(int(ingredient.ID))+"/icon", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)
	suite.assertStructuredError(w, handlers.CodeNotFound, "Иконка ингредиента не найдена")
}

func (suite *IngredientHandlerTestSuite) TestGetIngredientIcon_404_IngredientNotFound() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/42/icon", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)
	suite.assertStructuredError(w, handlers.CodeNotFound, "Ингредиент не найден")
}

func (suite *IngredientHandlerTestSuite) TestGetIngredientIcon_400_InvalidID() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/invalid/icon", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusBadRequest, w.Code)
	suite.assertStructuredError(w, handlers.CodeInvalidID, "Неверный ID ингредиента")
}

func (suite *IngredientHandlerTestSuite) TestCreateIngredient_201() {
	body := `{
		"name": "Джин",
		"description": "London dry gin",
		"unit_measurement": "мл",
		"abv": "крепкий",
		"ingredient_type": "крепкая часть"
	}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusCreated, w.Code)

	var response handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.NotZero(response.ID)
	suite.Equal("Джин", response.Name)
	suite.Equal("London dry gin", response.Description)
	suite.Equal(domain.UnitMl, response.UnitMeasurement)
	suite.Equal(domain.Strong, response.ABV)
	suite.Equal(domain.StrongPart, response.IngredientType)
	suite.False(response.HasIcon)
	suite.False(response.CreatedAt.IsZero())

	// Сверяем, что GET возвращает те же поля
	getW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/"+strconv.Itoa(int(response.ID)), nil)
	suite.router.ServeHTTP(getW, getReq)
	suite.Equal(http.StatusOK, getW.Code)

	var got handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(getW.Body.Bytes(), &got))
	suite.Equal(response.ID, got.ID)
	suite.Equal(response.Name, got.Name)
	suite.Equal(response.Description, got.Description)
	suite.Equal(response.UnitMeasurement, got.UnitMeasurement)
	suite.Equal(response.ABV, got.ABV)
	suite.Equal(response.IngredientType, got.IngredientType)
	suite.Equal(response.HasIcon, got.HasIcon)
	suite.True(response.CreatedAt.Equal(got.CreatedAt))
}

func (suite *IngredientHandlerTestSuite) TestCreateIngredient_201_WithoutDescription() {
	body := `{
		"name": "Тоник",
		"unit_measurement": "мл",
		"abv": "безалкогольный",
		"ingredient_type": "безалкогольная часть"
	}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusCreated, w.Code)

	var response handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.NotZero(response.ID)
	suite.Equal("Тоник", response.Name)
	suite.Empty(response.Description)
	suite.Equal(domain.UnitMl, response.UnitMeasurement)
	suite.Equal(domain.Free, response.ABV)
	suite.Equal(domain.FreePart, response.IngredientType)
	suite.False(response.HasIcon)

	getW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/"+strconv.Itoa(int(response.ID)), nil)
	suite.router.ServeHTTP(getW, getReq)
	suite.Equal(http.StatusOK, getW.Code)

	var got handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(getW.Body.Bytes(), &got))
	suite.Equal(response.ID, got.ID)
	suite.Equal(response.Name, got.Name)
	suite.Empty(got.Description)
	suite.Equal(response.UnitMeasurement, got.UnitMeasurement)
	suite.Equal(response.ABV, got.ABV)
	suite.Equal(response.IngredientType, got.IngredientType)
}

func (suite *IngredientHandlerTestSuite) TestCreateIngredient_400() {
	longName := strings.Repeat("а", 513)
	longDescription := strings.Repeat("б", 1025)

	cases := []struct {
		name       string
		body       string
		wantErrMsg string // пусто — достаточно любой непустой error
	}{
		{name: "bad json", body: `{`},
		{name: "missing name", body: `{
			"description": "London dry gin",
			"unit_measurement": "мл",
			"abv": "крепкий",
			"ingredient_type": "крепкая часть"
		}`},
		{name: "missing unit_measurement", body: `{
			"name": "Джин",
			"abv": "крепкий",
			"ingredient_type": "крепкая часть"
		}`},
		{name: "missing abv", body: `{
			"name": "Джин",
			"unit_measurement": "мл",
			"ingredient_type": "крепкая часть"
		}`},
		{name: "missing ingredient_type", body: `{
			"name": "Джин",
			"unit_measurement": "мл",
			"abv": "крепкий"
		}`},
		{name: "short name", body: `{
			"name": "ab",
			"unit_measurement": "мл",
			"abv": "крепкий",
			"ingredient_type": "крепкая часть"
		}`},
		{name: "name too long", body: `{
			"name": "` + longName + `",
			"unit_measurement": "мл",
			"abv": "крепкий",
			"ingredient_type": "крепкая часть"
		}`},
		{name: "description too long", body: `{
			"name": "Джин",
			"description": "` + longDescription + `",
			"unit_measurement": "мл",
			"abv": "крепкий",
			"ingredient_type": "крепкая часть"
		}`},
		{
			name: "invalid unit_measurement",
			body: `{
				"name": "Джин",
				"unit_measurement": "invalid",
				"abv": "крепкий",
				"ingredient_type": "крепкая часть"
			}`,
			wantErrMsg: "неверные данные ингредиента",
		},
		{
			name: "invalid abv",
			body: `{
				"name": "Джин",
				"unit_measurement": "мл",
				"abv": "invalid",
				"ingredient_type": "крепкая часть"
			}`,
			wantErrMsg: "неверные данные ингредиента",
		},
		{
			name: "invalid ingredient_type",
			body: `{
				"name": "Джин",
				"unit_measurement": "мл",
				"abv": "крепкий",
				"ingredient_type": "invalid"
			}`,
			wantErrMsg: "неверные данные ингредиента",
		},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)
			suite.assertStructuredErrorCode(w, handlers.CodeValidationError)
			if tt.wantErrMsg != "" {
				var response handlers.ErrorResponse
				suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
				suite.Contains(response.Error.Message, tt.wantErrMsg)
			}
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestCreateIngredient_409() {
	body := `{
		"name": "Джин",
		"description": "London dry gin",
		"unit_measurement": "мл",
		"abv": "крепкий",
		"ingredient_type": "крепкая часть"
	}`

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w1, req1)
	suite.Equal(http.StatusCreated, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ingredients", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w2, req2)

	suite.Equal(http.StatusConflict, w2.Code)
	suite.assertStructuredError(w2, handlers.CodeAlreadyExists, "запись уже существует")
}

func (suite *IngredientHandlerTestSuite) TestGetIngredient_400() {
	cases := []struct {
		name string
		path string
	}{
		{name: "not a number", path: "/api/v1/ingredients/not-a-number"},
		{name: "zero", path: "/api/v1/ingredients/0"},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)
			suite.assertStructuredError(w, handlers.CodeInvalidID, "Неверный ID ингредиента")
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestGetIngredient_404() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/42", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)
	suite.assertStructuredError(w, handlers.CodeNotFound, "Ингредиент не найден")
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
			request := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/"+strconv.Itoa(int(createdIngredient.ID)), nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusOK, w.Code)

			var response handlers.IngredientResponse
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

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_200() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "Старое описание",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	body := `{"version":1,"name":"Джин London Dry","description":"Новое описание"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal(created.ID, response.ID)
	suite.Equal("Джин London Dry", response.Name)
	suite.Equal("Новое описание", response.Description)
	suite.Equal(domain.UnitMl, response.UnitMeasurement)
	suite.Equal(domain.Strong, response.ABV)
	suite.Equal(domain.StrongPart, response.IngredientType)
	suite.False(response.HasIcon)
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_200_WithIcon() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Водка Absolut",
		Description:     "Старое описание",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
		Icon:            []byte{1, 2, 3},
	})
	suite.Require().NoError(err)

	body := `{"version":1,"description":"Знаменитый шведский бренд"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.True(response.HasIcon)

	newIcon, err := suite.repository.GetIcon(suite.ctx, created.ID)
	suite.Require().NoError(err)
	suite.Equal(created.Icon, newIcon)
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_200_SameName() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "Старое описание",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	body := `{"version":1,"name":"Джин"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal(created.ID, response.ID)
	suite.Equal(created.Name, response.Name)
	suite.Equal(created.Description, response.Description)
	suite.Equal(created.UnitMeasurement, response.UnitMeasurement)
	suite.Equal(created.ABV, response.ABV)
	suite.Equal(created.IngredientType, response.IngredientType)
	suite.False(response.HasIcon)
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_400_InvalidID() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/invalid",
		bytes.NewBufferString(`{"version":1,"name":"Джин London Dry"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusBadRequest, w.Code)
	suite.assertStructuredErrorCode(w, handlers.CodeInvalidID)
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_400() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "Описание",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	path := "/api/v1/ingredients/" + strconv.Itoa(int(created.ID))
	cases := []struct {
		name string
		body string
	}{
		{name: "empty body", body: `{}`},
		{name: "bad json", body: `{`},
		{name: "invalid enum", body: `{"version":1,"unit_measurement":"invalid"}`},
		{name: "empty name", body: `{"version":1,"name":""}`},
		{name: "short name", body: `{"version":1,"name":"ab"}`},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)
			suite.assertStructuredErrorCode(w, handlers.CodeValidationError)
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_404() {
	body := `{"version":1,"name":"Несуществующий"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/ingredients/42", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)
	suite.assertStructuredError(w, handlers.CodeNotFound, "Ингредиент не найден")
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_409() {
	_, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Ром",
		Description:     "Белый ром",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	vodka, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Водка",
		Description:     "Чистая водка",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	body := `{"version":1,"name":"Ром"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/"+strconv.Itoa(int(vodka.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusConflict, w.Code)
	suite.assertStructuredError(w, handlers.CodeAlreadyExists, "запись уже существует")
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_409_VersionConflict() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Текила",
		Description:     "Blanco",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	firstPatch := `{"version":1,"description":"Reposado"}`
	firstW := httptest.NewRecorder()
	firstReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(firstPatch),
	)
	firstReq.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(firstW, firstReq)
	suite.Equal(http.StatusOK, firstW.Code)

	stalePatch := `{"version":1,"description":"Anejo"}`
	staleW := httptest.NewRecorder()
	staleReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(stalePatch),
	)
	staleReq.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(staleW, staleReq)

	suite.Equal(http.StatusConflict, staleW.Code)
	suite.assertStructuredError(staleW, handlers.CodeVersionConflict, "конфликт версии ингредиента")
}

func (suite *IngredientHandlerTestSuite) TestDeleteIngredient_204() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "London dry gin",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodDelete,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID)),
		nil,
	)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNoContent, w.Code)
	suite.Empty(w.Body.Bytes())

	_, err = suite.repository.GetByID(suite.ctx, created.ID)
	suite.True(errors.Is(err, domain.ErrNotFound))
}

func (suite *IngredientHandlerTestSuite) TestDeleteIngredient_400() {
	cases := []struct {
		name string
		path string
	}{
		{name: "not a number", path: "/api/v1/ingredients/not-a-number"},
		{name: "zero", path: "/api/v1/ingredients/0"},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodDelete, tt.path, nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)
			suite.assertStructuredError(w, handlers.CodeInvalidID, "Неверный ID ингредиента")
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestDeleteIngredient_404() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/ingredients/42", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)
	suite.assertStructuredError(w, handlers.CodeNotFound, "Ингредиент не найден")
}

func (suite *IngredientHandlerTestSuite) TestSetIngredientIcon_204() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "London dry gin",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	icon := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/ingredients/"+strconv.Itoa(int(created.ID))+"/icon",
		bytes.NewReader(icon),
	)
	request.Header.Set("Content-Type", "application/octet-stream")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNoContent, w.Code)
	suite.Empty(w.Body.Bytes())

	updated, err := suite.repository.GetByID(suite.ctx, created.ID)
	suite.Require().NoError(err)
	suite.True(updated.HasIcon)

	newIcon, err := suite.repository.GetIcon(suite.ctx, created.ID)
	suite.Require().NoError(err)
	suite.Equal(icon, newIcon)

	getW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/ingredients/"+strconv.Itoa(int(created.ID)), nil)
	suite.router.ServeHTTP(getW, getReq)
	suite.Equal(http.StatusOK, getW.Code)

	var response handlers.IngredientResponse
	suite.Require().NoError(json.Unmarshal(getW.Body.Bytes(), &response))
	suite.True(response.HasIcon)
}

func (suite *IngredientHandlerTestSuite) TestSetIngredientIcon_400() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Ром",
		Description:     "Белый ром",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	path := "/api/v1/ingredients/" + strconv.Itoa(int(created.ID)) + "/icon"
	cases := []struct {
		name       string
		path       string
		body       []byte
		wantCode   string
		wantErrMsg string
	}{
		{name: "empty body", path: path, body: nil, wantCode: handlers.CodeValidationError, wantErrMsg: "иконка не может быть пустой"},
		{
			name:       "too large",
			path:       path,
			body:       make([]byte, services.MaxIconSize+1),
			wantCode:   handlers.CodeValidationError,
			wantErrMsg: "иконка слишком большая (макс. 512 KB)",
		},
		{
			name:       "invalid format",
			path:       path,
			body:       []byte("GIF89a"),
			wantCode:   handlers.CodeValidationError,
			wantErrMsg: "иконка должна быть в формате PNG или JPEG",
		},
		{name: "invalid id", path: "/api/v1/ingredients/not-a-number/icon", body: []byte{1}, wantCode: handlers.CodeInvalidID, wantErrMsg: "Неверный ID ингредиента"},
		{name: "zero id", path: "/api/v1/ingredients/0/icon", body: []byte{1}, wantCode: handlers.CodeInvalidID, wantErrMsg: "Неверный ID ингредиента"},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, tt.path, bytes.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/octet-stream")
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)
			suite.assertStructuredError(w, tt.wantCode, tt.wantErrMsg)
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestSetIngredientIcon_404() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/ingredients/42/icon",
		bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}),
	)
	request.Header.Set("Content-Type", "application/octet-stream")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)
	suite.assertStructuredError(w, handlers.CodeNotFound, "Ингредиент не найден")
}

func TestIngredientHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(IngredientHandlerTestSuite))
}
