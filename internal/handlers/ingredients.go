package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

// ListIngredients lista todos los ingredientes.
// ListIngredients lista todos los ingredientes disponibles
// @Summary Listar ingredientes
// @Description Obtiene la lista completa de ingredientes registrados en el sistema
// @Tags Ingredients
// @Produce json
// @Success 200 {array} models.Ingredient
// @Router /ingredients [get]
func ListIngredients(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ingredients []models.Ingredient
		if err := db.Find(&ingredients).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, ingredients)
	}
}

// GetIngredient obtiene un ingrediente por ID.
// GetIngredient obtiene un ingrediente por ID
// @Summary Obtener ingrediente
// @Description Obtiene los detalles de un ingrediente específico
// @Tags Ingredients
// @Produce json
// @Param id path int true "ID del ingrediente"
// @Success 200 {object} models.Ingredient
// @Failure 404 {object} map[string]string
// @Router /ingredients/{id} [get]
func GetIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var ingredient models.Ingredient
		if err := db.First(&ingredient, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ingrediente no encontrado"})
			return
		}
		c.JSON(http.StatusOK, ingredient)
	}
}

// CreateIngredient crea un nuevo ingrediente
// @Summary Crear ingrediente
// @Description Registra un nuevo insumo básico en el sistema
// @Tags Ingredients
// @Accept json
// @Produce json
// @Param input body models.Ingredient true "Datos del ingrediente"
// @Success 201 {object} models.Ingredient
// @Router /ingredients [post]
func CreateIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name     string  `json:"name" binding:"required"`
			Unit     string  `json:"unit" binding:"required"`
			UnitCost float64 `json:"unit_cost"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ingredient := models.Ingredient{
			Name:     input.Name,
			Unit:     input.Unit,
			UnitCost: input.UnitCost,
		}

		if err := db.Create(&ingredient).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, ingredient)
	}
}

// UpdateIngredient actualiza un ingrediente existente.
func UpdateIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var ingredient models.Ingredient
		if err := db.First(&ingredient, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ingrediente no encontrado"})
			return
		}

		var input struct {
			Name     string   `json:"name"`
			Unit     string   `json:"unit"`
			UnitCost *float64 `json:"unit_cost"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.Name != "" {
			ingredient.Name = input.Name
		}
		if input.Unit != "" {
			ingredient.Unit = input.Unit
		}
		if input.UnitCost != nil {
			ingredient.UnitCost = *input.UnitCost
		}

		if err := db.Save(&ingredient).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, ingredient)
	}
}

// DeleteIngredient elimina un ingrediente.
func DeleteIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var prods, stk int64
		db.Model(&models.ProductIngredient{}).Where("ingredient_id = ?", id).Count(&prods)
		db.Model(&models.StockIngredient{}).Where("ingredient_id = ?", id).Count(&stk)
		if prods > 0 || stk > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("No se puede eliminar: el ingrediente lo usan %d producto(s) y está en %d stock(s). Quitalo de ellos antes de borrarlo.", prods, stk),
			})
			return
		}

		if err := db.Delete(&models.Ingredient{}, id).Error; err != nil {
			// Si hay FKs (ON DELETE RESTRICT), GORM tirará error aquí
			c.JSON(http.StatusConflict, gin.H{"error": "No se puede eliminar el ingrediente porque está siendo usado en productos o stocks"})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}
