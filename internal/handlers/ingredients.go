package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"lingo-backend/internal/models"
)

// Insumos scoped al escenario (/scenarios/:scenario_id/ingredients).

func ListIngredients(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var items []models.Ingredient
		if err := db.Where("scenario_id = ?", c.Param("scenario_id")).Order("id").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func GetIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var it models.Ingredient
		if err := db.Where("scenario_id = ?", c.Param("scenario_id")).First(&it, c.Param("ingredient_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Insumo no encontrado"})
			return
		}
		c.JSON(http.StatusOK, it)
	}
}

type ingredientInput struct {
	Name           *string  `json:"name"`
	Unit           *string  `json:"unit"`
	UnitCost       *float64 `json:"unit_cost"`
	StockAvailable *float64 `json:"stock_available"`
}

func CreateIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		var in ingredientInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name == nil || *in.Name == "" || in.Unit == nil || *in.Unit == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name y unit son obligatorios"})
			return
		}
		it := models.Ingredient{ScenarioID: s.ID, Name: *in.Name, Unit: *in.Unit}
		applyIngredientInput(&it, in)
		if err := db.Create(&it).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se pudo crear (¿nombre duplicado en el escenario?)"})
			return
		}
		c.JSON(http.StatusCreated, it)
	}
}

func UpdateIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		var it models.Ingredient
		if err := db.Where("scenario_id = ?", s.ID).First(&it, c.Param("ingredient_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Insumo no encontrado"})
			return
		}
		var in ingredientInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name != nil {
			it.Name = *in.Name
		}
		if in.Unit != nil {
			it.Unit = *in.Unit
		}
		applyIngredientInput(&it, in)
		if err := db.Save(&it).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, it)
	}
}

func DeleteIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		if err := db.Where("scenario_id = ?", s.ID).Delete(&models.Ingredient{}, c.Param("ingredient_id")).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

func applyIngredientInput(it *models.Ingredient, in ingredientInput) {
	if in.UnitCost != nil {
		it.UnitCost = *in.UnitCost
	}
	if in.StockAvailable != nil {
		it.StockAvailable = *in.StockAvailable
	}
}
