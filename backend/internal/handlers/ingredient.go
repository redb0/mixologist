package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/services"
)

type IngredientController struct {
	service services.IngredientService
}

func NewIngredientController(service services.IngredientService) *IngredientController {
	return &IngredientController{service: service}
}

func (c *IngredientController) GetIngredients(ctx *gin.Context) {}

func (c *IngredientController) GetIngredient(ctx *gin.Context) {}

func (c *IngredientController) CreateIngredient(ctx *gin.Context) {
	var ingredientRequest CreateIngredientRequest
	if err := ctx.ShouldBindJSON(&ingredientRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ingredient, err := c.service.Create(
		ctx,
		ingredientRequest.Name,
		ingredientRequest.Description,
		ingredientRequest.UnitMeasurement,
		ingredientRequest.ABV,
		ingredientRequest.IngredientType,
	)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domain.ErrInvalidIngredientData) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ingredientResponse := CreateIngredientResponse{
		ID:              ingredient.ID,
		Name:            ingredient.Name,
		Description:     ingredient.Description,
		UnitMeasurement: ingredient.UnitMeasurement,
		ABV:             ingredient.ABV,
		IngredientType:  ingredient.IngredientType,
		CreatedAt:       ingredient.CreatedAt,
	}
	ctx.JSON(http.StatusCreated, ingredientResponse)
}

func (c *IngredientController) UpdateIngredient(ctx *gin.Context) {}

func (c *IngredientController) DeleteIngredient(ctx *gin.Context) {}
