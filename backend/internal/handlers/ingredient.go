package handlers

import (
	"errors"
	"net/http"
	"strconv"

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

func (c *IngredientController) GetIngredient(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID ингредиента"})
		return
	}
	ingredient, err := c.service.GetByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, toIngredientResponse(ingredient))
}

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
	ctx.JSON(http.StatusCreated, toIngredientResponse(ingredient))
}

func (c *IngredientController) UpdateIngredient(ctx *gin.Context) {}

func (c *IngredientController) DeleteIngredient(ctx *gin.Context) {}
