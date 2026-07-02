package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

func ListLingoLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var logs []models.LingoLog
		// Paginaría aquí en un caso real
		if err := db.Order("created_at desc").Limit(50).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, logs)
	}
}

func GetLingoLogsByJobID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("job_id")
		var logs []models.LingoLog
		if err := db.Where("job_id = ?", jobID).Order("created_at asc").Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, logs)
	}
}
