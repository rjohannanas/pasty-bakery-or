package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

func ListResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var resources []models.Resource
		if err := db.Find(&resources).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resources)
	}
}

func GetResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var resource models.Resource
		if err := db.Preload("Machines.Machine").Preload("OperationalResources").First(&resource, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso no encontrado"})
			return
		}
		c.JSON(http.StatusOK, resource)
	}
}

func CreateResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name        string  `json:"name" binding:"required"`
			Machines    []struct {
				MachineID      uint    `json:"machine_id" binding:"required"`
				HoursAvailable float64 `json:"hours_available" binding:"required"`
			} `json:"machines"`
			OperationalResources []struct {
				Name        string  `json:"name" binding:"required"`
				Available   float64 `json:"available" binding:"required"`
				CostPerUnit float64 `json:"cost_per_unit" binding:"required"`
			} `json:"operational_resources"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resource := models.Resource{
			Name:        input.Name,
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&resource).Error; err != nil {
				return err
			}
			for _, m := range input.Machines {
				rm := models.ResourceMachine{
					ResourceID:     resource.ID,
					MachineID:      m.MachineID,
					HoursAvailable: m.HoursAvailable,
				}
				if err := tx.Create(&rm).Error; err != nil {
					return err
				}
			}
			for _, opr := range input.OperationalResources {
				opRes := models.OperationalResource{
					ResourceID:  resource.ID,
					Name:        opr.Name,
					Available:   opr.Available,
					CostPerUnit: opr.CostPerUnit,
				}
				if err := tx.Create(&opRes).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		db.Preload("Machines.Machine").Preload("OperationalResources").First(&resource, resource.ID)
		c.JSON(http.StatusCreated, resource)
	}
}

func DeleteResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := db.Delete(&models.Resource{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}
