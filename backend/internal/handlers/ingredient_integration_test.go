package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	suite.router.GET("/ingredients", controller.ListIngredients)
	suite.router.GET("/ingredients/:id", controller.GetIngredient)
	suite.router.POST("/ingredients", controller.CreateIngredient)
	suite.router.PATCH("/ingredients/:id", controller.UpdateIngredient)
	suite.router.DELETE("/ingredients/:id", controller.DeleteIngredient)
	suite.router.PUT("/ingredients/:id/icon", controller.SetIngredientIcon)
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
	request := httptest.NewRequest(http.MethodGet, "/ingredients", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response []IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Empty(response)
	suite.Equal("[]", strings.TrimSpace(w.Body.String()))
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
	request := httptest.NewRequest(http.MethodGet, "/ingredients", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response []IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Require().Len(response, 2)

	// ORDER BY created_at DESC — последний созданный первым
	suite.Equal(withoutIcon.ID, response[0].ID)
	suite.Equal("Ром", response[0].Name)
	suite.Equal("Без иконки", response[0].Description)
	suite.Equal(domain.UnitMl, response[0].UnitMeasurement)
	suite.Equal(domain.Strong, response[0].ABV)
	suite.Equal(domain.StrongPart, response[0].IngredientType)
	suite.False(response[0].HasIcon)
	suite.False(response[0].CreatedAt.IsZero())

	suite.Equal(withIcon.ID, response[1].ID)
	suite.Equal("Джин", response[1].Name)
	suite.Equal("С иконкой", response[1].Description)
	suite.True(response[1].HasIcon)
	suite.False(response[1].CreatedAt.IsZero())
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
	request := httptest.NewRequest(http.MethodPost, "/ingredients", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusCreated, w.Code)

	var response IngredientResponse
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
	getReq := httptest.NewRequest(http.MethodGet, "/ingredients/"+strconv.Itoa(int(response.ID)), nil)
	suite.router.ServeHTTP(getW, getReq)
	suite.Equal(http.StatusOK, getW.Code)

	var got IngredientResponse
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
	request := httptest.NewRequest(http.MethodPost, "/ingredients", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusCreated, w.Code)

	var response IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.NotZero(response.ID)
	suite.Equal("Тоник", response.Name)
	suite.Empty(response.Description)
	suite.Equal(domain.UnitMl, response.UnitMeasurement)
	suite.Equal(domain.Free, response.ABV)
	suite.Equal(domain.FreePart, response.IngredientType)
	suite.False(response.HasIcon)

	getW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/ingredients/"+strconv.Itoa(int(response.ID)), nil)
	suite.router.ServeHTTP(getW, getReq)
	suite.Equal(http.StatusOK, getW.Code)

	var got IngredientResponse
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
			request := httptest.NewRequest(http.MethodPost, "/ingredients", bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)

			var response gin.H
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
			suite.NotEmpty(response["error"])
			if tt.wantErrMsg != "" {
				suite.Equal(tt.wantErrMsg, response["error"])
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
	req1 := httptest.NewRequest(http.MethodPost, "/ingredients", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w1, req1)
	suite.Equal(http.StatusCreated, w1.Code)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/ingredients", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w2, req2)

	suite.Equal(http.StatusConflict, w2.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w2.Body.Bytes(), &response))
	suite.Equal("запись уже существует", response["error"])
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

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_200() {
	created, err := suite.repository.Create(suite.ctx, &domain.Ingredient{
		Name:            "Джин",
		Description:     "Старое описание",
		UnitMeasurement: domain.UnitMl,
		ABV:             domain.Strong,
		IngredientType:  domain.StrongPart,
	})
	suite.Require().NoError(err)

	body := `{"name":"Джин London Dry","description":"Новое описание"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response IngredientResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal(created.ID, response.ID)
	suite.Equal("Джин London Dry", response.Name)
	suite.Equal("Новое описание", response.Description)
	suite.Equal(domain.UnitMl, response.UnitMeasurement)
	suite.Equal(domain.Strong, response.ABV)
	suite.Equal(domain.StrongPart, response.IngredientType)
	suite.False(response.HasIcon)
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

	body := `{"name":"Джин"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/ingredients/"+strconv.Itoa(int(created.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusOK, w.Code)

	var response IngredientResponse
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
		"/ingredients/invalid",
		bytes.NewBufferString(`{"name":"Джин London Dry"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusBadRequest, w.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.NotEmpty(response["error"])
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

	path := "/ingredients/" + strconv.Itoa(int(created.ID))
	cases := []struct {
		name string
		body string
	}{
		{name: "empty body", body: `{}`},
		{name: "bad json", body: `{`},
		{name: "invalid enum", body: `{"unit_measurement":"invalid"}`},
		{name: "empty name", body: `{"name":""}`},
		{name: "short name", body: `{"name":"ab"}`},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)

			var response gin.H
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
			suite.NotEmpty(response["error"])
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestUpdateIngredient_404() {
	body := `{"name":"Несуществующий"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/ingredients/42", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal("Ингредиент не найден", response["error"])
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

	body := `{"name":"Ром"}`
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch,
		"/ingredients/"+strconv.Itoa(int(vodka.ID)),
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusConflict, w.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal(response["error"], "запись уже существует")
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
		"/ingredients/"+strconv.Itoa(int(created.ID)),
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
		{name: "not a number", path: "/ingredients/not-a-number"},
		{name: "zero", path: "/ingredients/0"},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodDelete, tt.path, nil)
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)

			var response gin.H
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
			suite.Equal("неверный ID ингредиента", response["error"])
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestDeleteIngredient_404() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/ingredients/42", nil)
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal("Ингредиент не найден", response["error"])
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
		"/ingredients/"+strconv.Itoa(int(created.ID))+"/icon",
		bytes.NewReader(icon),
	)
	request.Header.Set("Content-Type", "application/octet-stream")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNoContent, w.Code)
	suite.Empty(w.Body.Bytes())

	updated, err := suite.repository.GetByID(suite.ctx, created.ID)
	suite.Require().NoError(err)
	suite.Equal(icon, updated.Icon)

	getW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/ingredients/"+strconv.Itoa(int(created.ID)), nil)
	suite.router.ServeHTTP(getW, getReq)
	suite.Equal(http.StatusOK, getW.Code)

	var response IngredientResponse
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

	path := "/ingredients/" + strconv.Itoa(int(created.ID)) + "/icon"
	cases := []struct {
		name       string
		path       string
		body       []byte
		wantErrMsg string
	}{
		{name: "empty body", path: path, body: nil, wantErrMsg: "иконка не может быть пустой"},
		{
			name:       "too large",
			path:       path,
			body:       make([]byte, services.MaxIconSize+1),
			wantErrMsg: "иконка слишком большая (макс. 512 KB)",
		},
		{
			name:       "invalid format",
			path:       path,
			body:       []byte("GIF89a"),
			wantErrMsg: "иконка должна быть в формате PNG или JPEG",
		},
		{name: "invalid id", path: "/ingredients/not-a-number/icon", body: []byte{1}, wantErrMsg: "неверный ID ингредиента"},
		{name: "zero id", path: "/ingredients/0/icon", body: []byte{1}, wantErrMsg: "неверный ID ингредиента"},
	}
	for _, tt := range cases {
		suite.Run(tt.name, func() {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, tt.path, bytes.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/octet-stream")
			suite.router.ServeHTTP(w, request)

			suite.Equal(http.StatusBadRequest, w.Code)

			var response gin.H
			suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
			suite.Equal(tt.wantErrMsg, response["error"])
		})
	}
}

func (suite *IngredientHandlerTestSuite) TestSetIngredientIcon_404() {
	w := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/ingredients/42/icon",
		bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}),
	)
	request.Header.Set("Content-Type", "application/octet-stream")
	suite.router.ServeHTTP(w, request)

	suite.Equal(http.StatusNotFound, w.Code)

	var response gin.H
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	suite.Equal("Ингредиент не найден", response["error"])
}

func TestIngredientHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(IngredientHandlerTestSuite))
}
