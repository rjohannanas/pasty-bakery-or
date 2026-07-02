package logger

import (
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

// SaveLingoLog persiste un registro de ejecución de LINGO en PostgreSQL.
func SaveLingoLog(db *gorm.DB, entry *models.LingoLog) {
	if err := db.Create(entry).Error; err != nil {
		L.Error().Err(err).Str("job_id", entry.JobID).Msg("[LINGO] no se pudo guardar LingoLog en DB")
	}
}
