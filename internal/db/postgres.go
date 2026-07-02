package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"lingo-backend/internal/models"
)

// Connect abre la conexión a PostgreSQL con reintentos y corre AutoMigrate.
func Connect(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	for attempt := 1; attempt <= 5; attempt++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			break
		}
		fmt.Printf("[DB] intento %d/5 fallido: %v — reintentando en 3s...\n", attempt, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a PostgreSQL tras 5 intentos: %w", err)
	}

	// AutoMigrate en orden correcto (tablas referenciadas primero)
	err = db.AutoMigrate(
		&models.Business{},
		&models.Product{},
		&models.Ingredient{},
		&models.ProductIngredient{},
		&models.Machine{},
		&models.ProductMachine{},
		&models.OperationalResource{},
		&models.ProductOperationalResource{},
		&models.Stock{},
		&models.StockIngredient{},
		&models.Resource{},
		&models.ResourceMachine{},
		&models.Optimization{},
		&models.OptimizationResult{},
		&models.LingoLog{},
	)
	if err != nil {
		return nil, fmt.Errorf("error en AutoMigrate: %w", err)
	}

	return db, nil
}
