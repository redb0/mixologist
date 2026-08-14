package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redb0/mixologist/internal/domain"
	"github.com/redb0/mixologist/internal/httperr"
	"github.com/redb0/mixologist/internal/services"
)

const ingredientIconCacheControl = "public, max-age=300, must-revalidate"

type IngredientController struct {
	service services.IngredientService
}

func NewIngredientController(service services.IngredientService) *IngredientController {
	return &IngredientController{service: service}
}

func (c *IngredientController) ListIngredients(ctx *gin.Context) {
	params, err := parseIngredientListParams(ctx)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	page, err := c.service.List(ctx, params)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toIngredientListResponse(page))
}

func (c *IngredientController) GetIngredient(ctx *gin.Context) {
	id, err := parseIngredientID(ctx)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ingredient, err := c.service.GetByID(ctx, id)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toIngredientResponsePtr(ingredient))
}

func (c *IngredientController) GetIngredientIcon(ctx *gin.Context) {
	id, err := parseIngredientID(ctx)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	icon, err := c.service.GetIcon(ctx, id)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ctx.Header("Content-Length", strconv.Itoa(len(icon)))
	ctx.Header("Cache-Control", ingredientIconCacheControl)
	ctx.Data(http.StatusOK, http.DetectContentType(icon), icon)
}

func (c *IngredientController) CreateIngredient(ctx *gin.Context) {
	var ingredientRequest CreateIngredientRequest
	if err := ctx.ShouldBindJSON(&ingredientRequest); err != nil {
		httperr.WriteError(ctx, domain.NewErrInvalidIngredientData("некорректные данные запроса"))
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
		httperr.WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, toIngredientResponsePtr(ingredient))
}

func (c *IngredientController) UpdateIngredient(ctx *gin.Context) {
	id, err := parseIngredientID(ctx)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	var req UpdateIngredientRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httperr.WriteError(ctx, domain.NewErrInvalidIngredientData("некорректные данные запроса"))
		return
	}

	ingredient, err := c.service.Update(ctx, id, services.UpdateIngredientPatch{
		ExpectedVersion: req.Version,
		Name:            req.Name,
		Description:     req.Description,
		UnitMeasurement: req.UnitMeasurement,
		ABV:             req.ABV,
		IngredientType:  req.IngredientType,
	})
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toIngredientResponsePtr(ingredient))
}

func (c *IngredientController) DeleteIngredient(ctx *gin.Context) {
	id, err := parseIngredientID(ctx)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	if err := c.service.Delete(ctx, id); err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (c *IngredientController) SetIngredientIcon(ctx *gin.Context) {
	id, err := parseIngredientID(ctx)
	if err != nil {
		httperr.WriteError(ctx, err)
		return
	}

	// Обрезаем чтение тела: oversized payload не грузится в память целиком.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, services.MaxIconSize)
	icon, err := ctx.GetRawData()
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			httperr.WriteError(ctx, domain.NewErrInvalidIngredientData("иконка слишком большая (макс. 512 KB)"))
			return
		}
		httperr.WriteError(ctx, domain.NewErrInvalidIngredientData("не удалось прочитать тело запроса"))
		return
	}

	if err := c.service.SetIcon(ctx, id, icon); err != nil {
		httperr.WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func parseIngredientID(ctx *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, domain.NewErrInvalidID("Неверный ID ингредиента")
	}
	return uint(id), nil
}
