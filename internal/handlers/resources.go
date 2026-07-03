package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

// GetDefaultResource devuelve el único Resource del sistema (lo crea si no existe).
// El negocio maneja un solo pool de recursos diarios (horas máquina + recursos
// operativos), no varios resources paralelos.
func GetDefaultResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var resource models.Resource
		err := db.Preload("Machines.Machine").Preload("OperationalResources").Order("id asc").First(&resource).Error
		if err == gorm.ErrRecordNotFound {
			resource = models.Resource{Name: "Recursos Diarios"}
			if err := db.Create(&resource).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			db.Preload("Machines.Machine").Preload("OperationalResources").First(&resource, resource.ID)
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resource)
	}
}

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
				HoursAvailable float64 `json:"hours_available"`
			} `json:"machines"`
			OperationalResources []struct {
				Name        string  `json:"name" binding:"required"`
				Available   float64 `json:"available"`
				CostPerUnit float64 `json:"cost_per_unit"`
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

// UpsertResourceMachine crea o actualiza las horas disponibles de una máquina
// dentro de un resource (upsert porque las horas del día se editan seguido y
// puede que la máquina todavía no esté vinculada a este resource).
func UpsertResourceMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("id")
		machineID := c.Param("machine_id")
		var input struct {
			// Sin binding:"required": 0 es válido (máquina recién dada de alta, aún
			// sin horas asignadas). Con required, GORM rechaza 0 y el auto-ligado
			// del front tira 400.
			HoursAvailable float64 `json:"hours_available"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var resource models.Resource
		if err := db.First(&resource, resourceID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso no encontrado"})
			return
		}
		mID, err := strconv.ParseUint(machineID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "machine_id inválido"})
			return
		}

		var rm models.ResourceMachine
		err = db.Where("resource_id = ? AND machine_id = ?", resourceID, machineID).First(&rm).Error
		if err == gorm.ErrRecordNotFound {
			rm = models.ResourceMachine{ResourceID: resource.ID, MachineID: uint(mID), HoursAvailable: input.HoursAvailable}
			if err := db.Create(&rm).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			rm.HoursAvailable = input.HoursAvailable
			if err := db.Save(&rm).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, rm)
	}
}

// RemoveResourceMachine desvincula una máquina del resource (no borra la
// Machine global, solo la fila de horas diarias).
func RemoveResourceMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("id")
		machineID := c.Param("machine_id")
		if err := db.Where("resource_id = ? AND machine_id = ?", resourceID, machineID).Delete(&models.ResourceMachine{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

// AddResourceOperationalResource agrega un nuevo recurso operativo (ej. electricidad,
// gas) a un resource existente.
func AddResourceOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("id")
		var input struct {
			Name        string  `json:"name" binding:"required"`
			Available   float64 `json:"available"`
			CostPerUnit float64 `json:"cost_per_unit"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var rID uint
		if err := db.Model(&models.Resource{}).Select("id").First(&rID, resourceID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso no encontrado"})
			return
		}

		opRes := models.OperationalResource{
			ResourceID:  rID,
			Name:        input.Name,
			Available:   input.Available,
			CostPerUnit: input.CostPerUnit,
		}
		if err := db.Create(&opRes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear recurso operativo"})
			return
		}
		c.JSON(http.StatusCreated, opRes)
	}
}

// UpdateResourceOperationalResource actualiza disponibilidad/costo de un recurso operativo.
func UpdateResourceOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("id")
		opresID := c.Param("opres_id")
		var input struct {
			Available   *float64 `json:"available"`
			CostPerUnit *float64 `json:"cost_per_unit"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var opRes models.OperationalResource
		if err := db.Where("resource_id = ? AND id = ?", resourceID, opresID).First(&opRes).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso operativo no encontrado"})
			return
		}
		if input.Available != nil {
			opRes.Available = *input.Available
		}
		if input.CostPerUnit != nil {
			opRes.CostPerUnit = *input.CostPerUnit
		}
		if err := db.Save(&opRes).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, opRes)
	}
}

// DeleteResourceOperationalResource elimina un recurso operativo de un resource.
func DeleteResourceOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("id")
		opresID := c.Param("opres_id")
		if err := db.Where("resource_id = ? AND id = ?", resourceID, opresID).Delete(&models.OperationalResource{}).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se puede eliminar el recurso operativo"})
			return
		}
		c.JSON(http.StatusNoContent, nil)
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
