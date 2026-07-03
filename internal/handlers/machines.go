package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

// ListMachines lista todas las máquinas.
func ListMachines(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var machines []models.Machine
		if err := db.Find(&machines).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, machines)
	}
}

// GetMachine obtiene una máquina por ID.
func GetMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var machine models.Machine
		if err := db.First(&machine, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Máquina no encontrada"})
			return
		}
		c.JSON(http.StatusOK, machine)
	}
}

// CreateMachine crea una nueva máquina.
func CreateMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name string `json:"name" binding:"required"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		machine := models.Machine{
			Name: input.Name,
		}

		if err := db.Create(&machine).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, machine)
	}
}

// UpdateMachine actualiza una máquina existente.
func UpdateMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var machine models.Machine
		if err := db.First(&machine, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Máquina no encontrada"})
			return
		}

		var input struct {
			Name string `json:"name"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.Name != "" {
			machine.Name = input.Name
		}

		if err := db.Save(&machine).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, machine)
	}
}

// DeleteMachine elimina una máquina.
func DeleteMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var prods, res int64
		db.Model(&models.ProductMachine{}).Where("machine_id = ?", id).Count(&prods)
		db.Model(&models.ResourceMachine{}).Where("machine_id = ?", id).Count(&res)
		if prods > 0 || res > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("No se puede eliminar: la máquina la usan %d producto(s) y %d recurso(s). Quitala de ellos antes de borrarla.", prods, res),
			})
			return
		}

		if err := db.Delete(&models.Machine{}, id).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se puede eliminar la máquina porque está siendo usada en productos o recursos"})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}
