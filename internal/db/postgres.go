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

	// AutoMigrate no altera constraints ya existentes. Forzamos ON DELETE CASCADE
	// en las FKs de la receta propia del producto (product_ingredients/machines/
	// operational_resources → products) para que borrar un producto se lleve su
	// receta (filas sin sentido sin el producto). Idempotente. Las FKs hacia
	// ingredients/machines siguen RESTRICT a propósito (son datos compartidos).
	if err := ensureProductRecipeCascade(db); err != nil {
		return nil, fmt.Errorf("error asegurando cascada de receta: %w", err)
	}

	return db, nil
}

// ensureProductRecipeCascade recrea las FKs de las tablas de receta hacia products
// con ON DELETE CASCADE. Idempotente: DROP IF EXISTS + ADD.
func ensureProductRecipeCascade(db *gorm.DB) error {
	stmts := []string{
		`ALTER TABLE product_ingredients DROP CONSTRAINT IF EXISTS fk_products_ingredients,
		 ADD CONSTRAINT fk_products_ingredients FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE`,
		`ALTER TABLE product_machines DROP CONSTRAINT IF EXISTS fk_products_machines,
		 ADD CONSTRAINT fk_products_machines FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE`,
		`ALTER TABLE product_operational_resources DROP CONSTRAINT IF EXISTS fk_products_operational_resources,
		 ADD CONSTRAINT fk_products_operational_resources FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
