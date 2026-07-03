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

	// Orden: escenario (contenedor) → dominio → recetas → corrida.
	// GORM crea las FKs con las acciones OnDelete de los tags del modelo.
	err = db.AutoMigrate(
		&models.Scenario{},
		&models.Product{},
		&models.Ingredient{},
		&models.Machine{},
		&models.OperationalResource{},
		&models.ProductIngredient{},
		&models.ProductMachine{},
		&models.ProductOperationalResource{},
		&models.Optimization{},
		&models.OptimizationResult{},
		&models.LingoLog{},
	)
	if err != nil {
		return nil, fmt.Errorf("error en AutoMigrate: %w", err)
	}

	// Deriva los invariantes de dominio (docs/03-invariants.md) a CHECK constraints:
	// el motor rechaza un estado ilegal aunque el código lo deje pasar.
	if err := ensureDomainChecks(db); err != nil {
		return nil, fmt.Errorf("error asegurando CHECK constraints: %w", err)
	}

	return db, nil
}

// ensureDomainChecks agrega los CHECK de dominio. Idempotente (DROP IF EXISTS + ADD).
func ensureDomainChecks(db *gorm.DB) error {
	type chk struct{ table, name, expr string }
	checks := []chk{
		{"products", "ck_products_nonneg", "sale_price >= 0 AND demand >= 0 AND min_batch >= 0 AND max_batch >= 0"},
		{"products", "ck_products_batch", "max_batch >= min_batch"}, // invariante M1
		{"ingredients", "ck_ingredients_nonneg", "unit_cost >= 0 AND stock_available >= 0"},
		{"machines", "ck_machines_nonneg", "hours_available >= 0"},
		{"operational_resources", "ck_opres_nonneg", "available >= 0 AND cost_per_unit >= 0"},
		{"product_ingredients", "ck_pi_nonneg", "quantity >= 0"},
		{"product_machines", "ck_pm_nonneg", "minutes_per_unit >= 0"},
		{"product_operational_resources", "ck_po_nonneg", "consumption_per_batch >= 0"},
		// Y(I) es @GIN (entero ≥0, nº de lotes), NO binario. Solo W(I) es @BIN.
		{"optimization_results", "ck_or_domain", "quantity_to_produce >= 0 AND batch_active >= 0 AND variety_flag >= 0"},
	}
	for _, c := range checks {
		stmt := fmt.Sprintf(
			"ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s, ADD CONSTRAINT %s CHECK (%s)",
			c.table, c.name, c.name, c.expr,
		)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}
	return nil
}
