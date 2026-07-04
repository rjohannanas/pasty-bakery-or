package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"lingo-backend/internal/models"
	"lingo-backend/internal/queue"
)

// El disparo de optimización vive en scenarios.go (OptimizeScenario): congela el
// escenario y encola. Acá quedan la consulta de estado y los resultados.

// GetJobStatus consulta el estado de un job (Redis + Postgres).
func GetJobStatus(db *gorm.DB, q *queue.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("job_id")
		status, err := q.GetStatus(c.Request.Context(), jobID)
		if err != nil {
			// Fallback a Postgres si Redis ya no lo tiene.
			var opt models.Optimization
			if e := db.Where("job_id = ?", jobID).First(&opt).Error; e != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Job no encontrado"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"job_id": jobID, "status": opt.Status, "id": opt.ID})
			return
		}
		var opt models.Optimization
		db.Select("id").Where("job_id = ?", jobID).First(&opt)
		c.JSON(http.StatusOK, gin.H{"job_id": jobID, "status": status, "id": opt.ID})
	}
}

// ListOptimizations muestra el historial de corridas.
func ListOptimizations(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var opts []models.Optimization
		if err := db.Order("created_at desc").Find(&opts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, opts)
	}
}

// GetOptimizationResult retorna una corrida con su plan de producción.
func GetOptimizationResult(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var opt models.Optimization
		if err := db.
			Preload("Results").
			Preload("Scenario").
			// Config del escenario para los gráficos "uso vs capacidad" (necesita las
			// matrices de receta Q/T/CM y los disponibles IN/CAP/DISP junto con X/Y).
			Preload("Scenario.Machines").
			Preload("Scenario.Ingredients").
			Preload("Scenario.OperationalResources").
			Preload("Scenario.Products.Machines.Machine").
			Preload("Scenario.Products.Ingredients.Ingredient").
			Preload("Scenario.Products.OperationalResources.OperationalResource").
			First(&opt, c.Param("id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Optimización no encontrada"})
			return
		}
		c.JSON(http.StatusOK, opt)
	}
}

// GetQueueStatus expone el estado de la cola.
func GetQueueStatus(q *queue.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		pending, err := q.ListPending(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		all, _ := q.GetAllJobsStatus(c.Request.Context())
		processing := []string{}
		for id, st := range all {
			if st == string(models.StatusProcessing) {
				processing = append(processing, id)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"pending_count":    len(pending),
			"pending_ids":      pending,
			"processing_count": len(processing),
			"processing_ids":   processing,
		})
	}
}
