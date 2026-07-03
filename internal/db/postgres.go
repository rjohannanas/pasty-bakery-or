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

	// Invariante M2: una celda de receta y su entidad comparten escenario.
	// AutoMigrate no arma FKs compuestas, se agregan por SQL.
	if err := ensureRecipeCompositeFK(db); err != nil {
		return nil, fmt.Errorf("error asegurando FK compuestas de receta: %w", err)
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
		{"machines", "ck_machines_nonneg", "capacity_minutes >= 0"},
		{"operational_resources", "ck_opres_nonneg", "available >= 0 AND cost_per_unit >= 0"},
		{"product_ingredients", "ck_pi_nonneg", "quantity >= 0"},
		{"product_machines", "ck_pm_nonneg", "minutes_per_unit >= 0"},
		{"product_operational_resources", "ck_po_nonneg", "consumption_per_batch >= 0"},
		// Y(I) es @GIN (entero ≥0, nº de lotes), NO binario. Solo W(I) es @BIN.
		{"optimization_results", "ck_or_domain", "quantity_to_produce >= 0 AND batch_active >= 0 AND variety_flag >= 0"},
		// Invariante A3: nombres (y unidad) no vacíos, a nivel motor.
		{"scenarios", "ck_scenarios_name", "length(btrim(name)) > 0"},
		{"products", "ck_products_name", "length(btrim(name)) > 0"},
		{"ingredients", "ck_ingredients_name", "length(btrim(name)) > 0"},
		{"ingredients", "ck_ingredients_unit", "length(btrim(unit)) > 0"},
		{"machines", "ck_machines_name", "length(btrim(name)) > 0"},
		{"operational_resources", "ck_opres_name", "length(btrim(name)) > 0"},
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

// ensureRecipeCompositeFK deriva el invariante M2 a FKs compuestas: una celda de
// receta (Q/T/CM) y la entidad que referencia deben pertenecer al MISMO escenario.
// La FK compuesta apunta a (scenario_id, id) de la entidad, así el motor rechaza
// pegar un insumo/máquina/opres de otro escenario. Idempotente (DROP IF EXISTS + ADD).
// ON DELETE CASCADE mantiene L2 (borrar la entidad cascadea su celda de receta).
func ensureRecipeCompositeFK(db *gorm.DB) error {
	type fk struct{ table, name, cols, ref, refCols string }
	fks := []fk{
		{"product_ingredients", "fk_pi_scen_product", "(scenario_id, product_id)", "products", "(scenario_id, id)"},
		{"product_ingredients", "fk_pi_scen_ingredient", "(scenario_id, ingredient_id)", "ingredients", "(scenario_id, id)"},
		{"product_machines", "fk_pm_scen_product", "(scenario_id, product_id)", "products", "(scenario_id, id)"},
		{"product_machines", "fk_pm_scen_machine", "(scenario_id, machine_id)", "machines", "(scenario_id, id)"},
		{"product_operational_resources", "fk_po_scen_product", "(scenario_id, product_id)", "products", "(scenario_id, id)"},
		{"product_operational_resources", "fk_po_scen_opres", "(scenario_id, operational_resource_id)", "operational_resources", "(scenario_id, id)"},
	}

	// 1. Dropear las FK compuestas PRIMERO: dependen de los UNIQUE de abajo, así que
	//    hay que sacarlas antes de poder recrear el UNIQUE (idempotencia en restart).
	for _, f := range fks {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", f.table, f.name)).Error; err != nil {
			return fmt.Errorf("drop %s: %w", f.name, err)
		}
	}

	// 2. UNIQUE(scenario_id, id) en las entidades referidas: requisito para que la
	//    FK compuesta pueda apuntarles. Ya sin dependientes, drop+add es seguro.
	uniques := []struct{ table, name string }{
		{"products", "uq_products_scen_id"},
		{"ingredients", "uq_ingredients_scen_id"},
		{"machines", "uq_machines_scen_id"},
		{"operational_resources", "uq_opres_scen_id"},
	}
	for _, u := range uniques {
		stmt := fmt.Sprintf(
			"ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s, ADD CONSTRAINT %s UNIQUE (scenario_id, id)",
			u.table, u.name, u.name,
		)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
	}

	// 3. Recrear las FK compuestas (una por matriz de receta hacia sus dos entidades).
	for _, f := range fks {
		stmt := fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY %s REFERENCES %s %s ON DELETE CASCADE",
			f.table, f.name, f.cols, f.ref, f.refCols,
		)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return nil
}
