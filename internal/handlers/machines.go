package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"lingo-backend/internal/models"
)

// Máquinas scoped al escenario (/scenarios/:scenario_id/machines).

func ListMachines(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var items []models.Machine
		if err := db.Where("scenario_id = ?", c.Param("scenario_id")).Order("id").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func GetMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var it models.Machine
		if err := db.Where("scenario_id = ?", c.Param("scenario_id")).First(&it, c.Param("machine_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Máquina no encontrada"})
			return
		}
		c.JSON(http.StatusOK, it)
	}
}

type machineInput struct {
	Name            *string  `json:"name"`
	CapacityMinutes *float64 `json:"capacity_minutes"`
}

func CreateMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		var in machineInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name == nil || *in.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name es obligatorio"})
			return
		}
		it := models.Machine{ScenarioID: s.ID, Name: *in.Name}
		if in.CapacityMinutes != nil {
			it.CapacityMinutes = *in.CapacityMinutes
		}
		if err := db.Create(&it).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se pudo crear (¿nombre duplicado en el escenario?)"})
			return
		}
		c.JSON(http.StatusCreated, it)
	}
}

func UpdateMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		var it models.Machine
		if err := db.Where("scenario_id = ?", s.ID).First(&it, c.Param("machine_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Máquina no encontrada"})
			return
		}
		var in machineInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name != nil {
			it.Name = *in.Name
		}
		if in.CapacityMinutes != nil {
			it.CapacityMinutes = *in.CapacityMinutes
		}
		if err := db.Save(&it).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, it)
	}
}

func DeleteMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok || !requireDraft(c, s) {
			return
		}
		if err := db.Where("scenario_id = ?", s.ID).Delete(&models.Machine{}, c.Param("machine_id")).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}
