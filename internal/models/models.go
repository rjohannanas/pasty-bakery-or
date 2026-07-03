package models

import "time"

// Modelo derivado de docs/02-data-dictionary.md (autoritativo).
// Arquitectura: escenarios instanciados + fork. Cada entidad de dominio
// pertenece a un escenario y guarda sus parámetros. La identidad se archiva,
// nunca se hard-deletea. Ver docs/03-invariants.md y docs/adr/.

// ─── Scenario ─────────────────────────────────────────────────────────────────

type ScenarioStatus string

const (
	ScenarioDraft    ScenarioStatus = "draft"    // editable
	ScenarioFrozen   ScenarioStatus = "frozen"   // ya optimizado, inmutable
	ScenarioArchived ScenarioStatus = "archived" // oculto, conserva historia
)

// Scenario es un plan completo y editable: la unidad clonable del what-if.
type Scenario struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"not null;uniqueIndex" json:"name"`
	Notes         string         `json:"notes,omitempty"`
	Status        ScenarioStatus `gorm:"type:varchar(20);not null;default:'draft'" json:"status"`
	IsBase        bool           `gorm:"not null;default:false" json:"is_base"`
	ParentID      *uint          `json:"parent_id,omitempty"`                        // forkeado de
	MaxProduction float64        `gorm:"not null;default:200" json:"max_production"` // M
	MinVariety    int            `gorm:"not null;default:7" json:"min_variety"`      // PRO

	// Dominio instanciado (CASCADE: borrar el escenario se lleva sus entidades).
	Products             []Product             `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"products,omitempty"`
	Ingredients          []Ingredient          `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"ingredients,omitempty"`
	Machines             []Machine             `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"machines,omitempty"`
	OperationalResources []OperationalResource `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"operational_resources,omitempty"`

	Parent    *Scenario `gorm:"foreignKey:ParentID;constraint:OnDelete:SET NULL" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── Entidades de dominio (instanciadas por escenario) ─────────────────────────

// Product — LINGO: P (sale_price), D (demand), LI (min_batch), LS (max_batch).
type Product struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	ScenarioID  uint    `gorm:"not null;index;uniqueIndex:uq_scen_prod" json:"scenario_id"`
	CanonicalID *uint   `gorm:"index" json:"canonical_id,omitempty"` // identidad cruzada
	Name        string  `gorm:"not null;uniqueIndex:uq_scen_prod" json:"name"`
	SalePrice   float64 `gorm:"not null;default:0" json:"sale_price"` // P
	Demand      float64 `gorm:"not null;default:0" json:"demand"`     // D
	MinBatch    float64 `gorm:"not null;default:0" json:"min_batch"`  // LI
	MaxBatch    float64 `gorm:"not null;default:0" json:"max_batch"`  // LS (>= min_batch)

	Ingredients          []ProductIngredient          `gorm:"foreignKey:ProductID" json:"ingredients,omitempty"`
	Machines             []ProductMachine             `gorm:"foreignKey:ProductID" json:"machines,omitempty"`
	OperationalResources []ProductOperationalResource `gorm:"foreignKey:ProductID" json:"operational_resources,omitempty"`

	Scenario  Scenario  `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"-"`
	Canonical *Product  `gorm:"foreignKey:CanonicalID;constraint:OnDelete:SET NULL" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Ingredient — LINGO: CU (unit_cost), IN (stock_available). Absorbe StockIngredient.
type Ingredient struct {
	ID             uint        `gorm:"primaryKey" json:"id"`
	ScenarioID     uint        `gorm:"not null;index;uniqueIndex:uq_scen_ing" json:"scenario_id"`
	CanonicalID    *uint       `gorm:"index" json:"canonical_id,omitempty"`
	Name           string      `gorm:"not null;uniqueIndex:uq_scen_ing" json:"name"`
	Unit           string      `gorm:"not null" json:"unit"`
	UnitCost       float64     `gorm:"not null;default:0" json:"unit_cost"`       // CU
	StockAvailable float64     `gorm:"not null;default:0" json:"stock_available"` // IN
	Scenario       Scenario    `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"-"`
	Canonical      *Ingredient `gorm:"foreignKey:CanonicalID;constraint:OnDelete:SET NULL" json:"-"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// Machine — LINGO: CAP (capacity_minutes; misma unidad que T, sin conversión).
// Absorbe ResourceMachine.
type Machine struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ScenarioID      uint      `gorm:"not null;index;uniqueIndex:uq_scen_mach" json:"scenario_id"`
	CanonicalID     *uint     `gorm:"index" json:"canonical_id,omitempty"`
	Name            string    `gorm:"not null;uniqueIndex:uq_scen_mach" json:"name"`
	CapacityMinutes float64   `gorm:"not null;default:0" json:"capacity_minutes"` // CAP (minutos)
	Scenario        Scenario  `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"-"`
	Canonical       *Machine  `gorm:"foreignKey:CanonicalID;constraint:OnDelete:SET NULL" json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// OperationalResource — LINGO: DISP (available), CR (cost_per_unit).
type OperationalResource struct {
	ID          uint                 `gorm:"primaryKey" json:"id"`
	ScenarioID  uint                 `gorm:"not null;index;uniqueIndex:uq_scen_opres" json:"scenario_id"`
	CanonicalID *uint                `gorm:"index" json:"canonical_id,omitempty"`
	Name        string               `gorm:"not null;uniqueIndex:uq_scen_opres" json:"name"`
	Available   float64              `gorm:"not null;default:0" json:"available"`     // DISP
	CostPerUnit float64              `gorm:"not null;default:0" json:"cost_per_unit"` // CR
	Scenario    Scenario             `gorm:"foreignKey:ScenarioID;constraint:OnDelete:CASCADE" json:"-"`
	Canonical   *OperationalResource `gorm:"foreignKey:CanonicalID;constraint:OnDelete:SET NULL" json:"-"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// ─── Recetas (matrices Q / T / CM) ─────────────────────────────────────────────
// Llevan ScenarioID explícito para la FK compuesta (mismo escenario), que se
// agrega por migración SQL (AutoMigrate no arma FKs compuestas). Ver
// db.ensureRecipeCompositeFK.

// ProductIngredient — Q(I,J).
type ProductIngredient struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	ScenarioID   uint       `gorm:"not null;index" json:"scenario_id"`
	ProductID    uint       `gorm:"not null;uniqueIndex:uq_prod_ing" json:"product_id"`
	IngredientID uint       `gorm:"not null;uniqueIndex:uq_prod_ing" json:"ingredient_id"`
	Quantity     float64    `gorm:"not null;default:0" json:"quantity"` // Q
	Product      Product    `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE" json:"-"`
	Ingredient   Ingredient `gorm:"foreignKey:IngredientID;constraint:OnDelete:CASCADE" json:"ingredient,omitempty"`
}

// ProductMachine — T(I,K).
type ProductMachine struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	ScenarioID     uint    `gorm:"not null;index" json:"scenario_id"`
	ProductID      uint    `gorm:"not null;uniqueIndex:uq_prod_mach" json:"product_id"`
	MachineID      uint    `gorm:"not null;uniqueIndex:uq_prod_mach" json:"machine_id"`
	MinutesPerUnit float64 `gorm:"not null;default:0" json:"minutes_per_unit"` // T
	Product        Product `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE" json:"-"`
	Machine        Machine `gorm:"foreignKey:MachineID;constraint:OnDelete:CASCADE" json:"machine,omitempty"`
}

// ProductOperationalResource — CM(I,R).
type ProductOperationalResource struct {
	ID                    uint                `gorm:"primaryKey" json:"id"`
	ScenarioID            uint                `gorm:"not null;index" json:"scenario_id"`
	ProductID             uint                `gorm:"not null;uniqueIndex:uq_prod_opres" json:"product_id"`
	OperationalResourceID uint                `gorm:"not null;uniqueIndex:uq_prod_opres" json:"operational_resource_id"`
	ConsumptionPerBatch   float64             `gorm:"not null;default:0" json:"consumption_per_batch"` // CM
	Product               Product             `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE" json:"-"`
	OperationalResource   OperationalResource `gorm:"foreignKey:OperationalResourceID;constraint:OnDelete:CASCADE" json:"operational_resource,omitempty"`
}

// ─── Corrida ───────────────────────────────────────────────────────────────────

type OptimizationStatus string

const (
	StatusPending    OptimizationStatus = "pending"
	StatusProcessing OptimizationStatus = "processing"
	StatusDone       OptimizationStatus = "done"
	StatusError      OptimizationStatus = "error"
	StatusCancelled  OptimizationStatus = "cancelled"
)

// Optimization es una ejecución del solver sobre un escenario congelado.
type Optimization struct {
	ID            uint               `gorm:"primaryKey" json:"id"`
	ScenarioID    *uint              `gorm:"index" json:"scenario_id,omitempty"` // SET NULL al archivar
	JobID         string             `gorm:"not null;uniqueIndex" json:"job_id"`
	Status        OptimizationStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`
	MaxProduction float64            `gorm:"not null;default:200" json:"max_production"` // M efectivo
	MinVariety    int                `gorm:"not null;default:7" json:"min_variety"`      // PRO efectivo
	TotalProfit   float64            `json:"total_profit"`

	Scenario   *Scenario            `gorm:"foreignKey:ScenarioID;constraint:OnDelete:SET NULL" json:"scenario,omitempty"`
	Results    []OptimizationResult `gorm:"foreignKey:OptimizationID" json:"results,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
	StartedAt  *time.Time           `json:"started_at,omitempty"`
	FinishedAt *time.Time           `json:"finished_at,omitempty"`
}

// OptimizationResult es autocontenido: guarda el nombre denormalizado para que el
// plan histórico se lea aunque el producto se archive. LINGO: X, Y, W.
type OptimizationResult struct {
	ID                 uint    `gorm:"primaryKey" json:"id"`
	OptimizationID     uint    `gorm:"not null;index" json:"optimization_id"`
	ProductID          *uint   `gorm:"index" json:"product_id,omitempty"`             // link blando
	CanonicalProductID *uint   `gorm:"index" json:"canonical_product_id,omitempty"`   // analítica cruzada
	ProductName        string  `gorm:"not null" json:"product_name"`                  // denormalizado
	QuantityToProduce  float64 `gorm:"not null;default:0" json:"quantity_to_produce"` // X
	BatchActive        float64 `gorm:"not null;default:0" json:"batch_active"`        // Y
	VarietyFlag        float64 `gorm:"not null;default:0" json:"variety_flag"`        // W
	ExpectedProfit     float64 `gorm:"not null;default:0" json:"expected_profit"`

	Optimization Optimization `gorm:"foreignKey:OptimizationID;constraint:OnDelete:CASCADE" json:"-"`
	Product      *Product     `gorm:"foreignKey:ProductID;constraint:OnDelete:SET NULL" json:"product,omitempty"`
}

// ─── LingoLog ───────────────────────────────────────────────────────────────────

type LingoLog struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	JobID          string    `gorm:"not null;index" json:"job_id"`
	OptimizationID uint      `gorm:"index" json:"optimization_id"`
	Level          string    `gorm:"type:varchar(10);not null" json:"level"` // info | error
	Message        string    `gorm:"not null" json:"message"`
	ModelGenerated string    `gorm:"type:text" json:"model_generated,omitempty"`
	LingoOutput    string    `gorm:"type:text" json:"lingo_output,omitempty"`
	DurationMs     int64     `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}
