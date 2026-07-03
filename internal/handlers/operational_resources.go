package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"lingo-backend/internal/models"
)

// Recursos operativos scoped al escenario
// (/scenarios/:scenario_id/operational-resources).

func ListOperationalResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var items []models.OperationalResource
		if err := db.Where("scenario_id = ?", c.Param("scenario_id")).Order("id").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func GetOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var it models.OperationalResource
		if err := db.Where("scenario_id = ?", c.Param("scenario_id")).First(&it, c.Param("opres_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso operativo no encontrado"})
			return
		}
		c.JSON(http.StatusOK, it)
	}
}

type opresInput struct {
	Name        *string  `json:"name"`
	Available   *float64 `json:"available"`
	CostPerUnit *float64 `json:"cost_per_unit"`
}

func CreateOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		var in opresInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name == nil || *in.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name es obligatorio"})
			return
		}
		it := models.OperationalResource{ScenarioID: s.ID, Name: *in.Name}
		applyOpresInput(&it, in)
		if err := db.Create(&it).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se pudo crear (¿nombre duplicado en el escenario?)"})
			return
		}
		c.JSON(http.StatusCreated, it)
	}
}

func UpdateOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		var it models.OperationalResource
		if err := db.Where("scenario_id = ?", s.ID).First(&it, c.Param("opres_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso operativo no encontrado"})
			return
		}
		var in opresInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name != nil {
			it.Name = *in.Name
		}
		applyOpresInput(&it, in)
		if err := db.Save(&it).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, it)
	}
}

func DeleteOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		if err := db.Where("scenario_id = ?", s.ID).Delete(&models.OperationalResource{}, c.Param("opres_id")).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

func applyOpresInput(it *models.OperationalResource, in opresInput) {
	if in.Available != nil {
		it.Available = *in.Available
	}
	if in.CostPerUnit != nil {
		it.CostPerUnit = *in.CostPerUnit
	}
}
