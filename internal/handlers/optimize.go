package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	
	"lingo-backend/internal/models"
	"lingo-backend/internal/queue"
)

// Optimize triggering logic
// Optimize inicia un nuevo proceso de cálculo matemático
// @Summary Iniciar optimización
// @Description Encola un nuevo trabajo para que LINGO calcule la producción óptima
// @Tags Optimization
// @Accept json
// @Produce json
// @Param output body models.Optimization true "IDs de Stock y Recursos"
// @Success 202 {object} map[string]string "job_id, status"
// @Router /optimize [post]
func Optimize(db *gorm.DB, q *queue.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			StockID       uint    `json:"stock_id" binding:"required"`
			ResourceID    uint    `json:"resource_id" binding:"required"`
			MaxProduction float64 `json:"max_production"`
			MinVariety    int     `json:"min_variety"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.MaxProduction <= 0 {
			input.MaxProduction = 200
		}
		if input.MinVariety <= 0 {
			input.MinVariety = 7
		}

		// 1. Validar que stock y recursos existen en Postgres
		var stock models.Stock
		if err := db.First(&stock, input.StockID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stock no encontrado"})
			return
		}
		var resource models.Resource
		if err := db.First(&resource, input.ResourceID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Recurso no encontrado"})
			return
		}

		// 1b. Evitar ejecuciones duplicadas simultáneas
		var activeCount int64
		db.Model(&models.Optimization{}).Where("stock_id = ? AND resource_id = ? AND status IN (?, ?)",
			input.StockID, input.ResourceID, models.StatusPending, models.StatusProcessing).Count(&activeCount)
		if activeCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya existe una optimización en curso para este stock y recurso"})
			return
		}

		// 2. Crear registro en Postgres
		jobID := uuid.New().String()
		opt := models.Optimization{
			StockID:       input.StockID,
			ResourceID:    input.ResourceID,
			Status:        models.StatusPending,
			JobID:         jobID,
			MaxProduction: input.MaxProduction,
			MinVariety:    input.MinVariety,
		}
		if err := db.Create(&opt).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 3. Encolar en Redis
		if err := q.PushJob(c.Request.Context(), jobID); err != nil {
			// Si no se pudo encolar, no dejamos la fila huérfana en pending.
			db.Delete(&opt)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al encolar trabajo en Redis"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"job_id": jobID,
			"status": "pending",
		})
	}
}

// GetJobStatus consulta el estado actual del job
// GetJobStatus consulta el estado actual de un procesamiento en Redis y Postgres
// @Summary Consultar estado de Job
// @Description Obtiene el estado (pending, processing, done, error) de un trabajo por su UUID y su ID de base de datos
// @Tags Optimization
// @Produce json
// @Param job_id path string true "UUID del trabajo"
// @Success 200 {object} map[string]interface{} "job_id, status, id"
// @Router /optimize/{job_id} [get]
func GetJobStatus(db *gorm.DB, q *queue.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("job_id")
		status, err := q.GetStatus(c.Request.Context(), jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job no encontrado"})
			return
		}
		
		var opt models.Optimization
		db.Select("id").Where("job_id = ?", jobID).First(&opt)
		
		c.JSON(http.StatusOK, gin.H{
			"job_id": jobID,
			"status": status,
			"id":     opt.ID,
		})
	}
}

// ListOptimizations muestra el historial
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

// GetOptimizationResult retorna los resultados numéricos de una optimización
// GetOptimizationResult retorna los resultados numéricos detallados
// @Summary Obtener resultados de optimización
// @Description Obtiene las cantidades óptimas a producir y ganancias esperadas
// @Tags Optimization
// @Produce json
// @Param id path int true "ID de la optimización (Postgres)"
// @Success 200 {object} models.Optimization
// @Router /results/{id} [get]
func GetOptimizationResult(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var opt models.Optimization
		if err := db.
			Preload("Results.Product.Machines.Machine").
			Preload("Stock.Ingredients.Ingredient").
			Preload("Resource.Machines.Machine").
			Preload("Resource.OperationalResources").
			First(&opt, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Optimización no encontrada"})
			return
		}
		c.JSON(http.StatusOK, opt)
	}
}

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
			"pending_count": len(pending),
			"pending_ids":   pending,
			"processing_count": len(processing),
			"processing_ids":   processing,
		})
	}
}
