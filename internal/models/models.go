package models

import (
	"encoding/json"
	"time"
)

// ─── Business ───────────────────────────────────────────────────────────────

type Business struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Product ─────────────────────────────────────────────────────────────────

type Product struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;uniqueIndex" json:"name"`
	SalePrice float64   `gorm:"not null" json:"sale_price"`
	Demand    float64   `gorm:"not null;default:0" json:"demand"`
	MinBatch  float64   `gorm:"not null;default:0" json:"min_batch"`
	MaxBatch  float64   `gorm:"not null;default:0" json:"max_batch"`
	Ingredients []ProductIngredient `gorm:"foreignKey:ProductID" json:"ingredients,omitempty"`
	Machines    []ProductMachine    `gorm:"foreignKey:ProductID" json:"machines,omitempty"`
	OperationalResources []ProductOperationalResource `gorm:"foreignKey:ProductID" json:"operational_resources,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── Ingredient ──────────────────────────────────────────────────────────────

type Ingredient struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;uniqueIndex" json:"name"`
	Unit      string    `gorm:"not null" json:"unit"` // kg, litros, unidades, etc.
	UnitCost  float64   `gorm:"not null;default:0" json:"unit_cost"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── ProductIngredient (bridge) ───────────────────────────────────────────────

type ProductIngredient struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	ProductID    uint       `gorm:"not null;uniqueIndex:uq_prod_ing" json:"product_id"`
	IngredientID uint       `gorm:"not null;uniqueIndex:uq_prod_ing" json:"ingredient_id"`
	Quantity     float64    `gorm:"not null" json:"quantity"`
	Product      Product    `gorm:"foreignKey:ProductID;constraint:OnDelete:RESTRICT" json:"-"`
	Ingredient   Ingredient `gorm:"foreignKey:IngredientID;constraint:OnDelete:RESTRICT" json:"ingredient,omitempty"`
}

// ─── Machine ──────────────────────────────────────────────────────────────────

type Machine struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;uniqueIndex" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── ProductMachine ───────────────────────────────────────────────────────────

type ProductMachine struct {
	ID             uint    `gorm:"primaryKey" json:"id"`
	ProductID      uint    `gorm:"not null;uniqueIndex:uq_prod_mach" json:"product_id"`
	MachineID      uint    `gorm:"not null;uniqueIndex:uq_prod_mach" json:"machine_id"`
	MinutesPerUnit float64 `gorm:"not null" json:"minutes_per_unit"`
	Product        Product `gorm:"foreignKey:ProductID;constraint:OnDelete:RESTRICT" json:"-"`
	Machine        Machine `gorm:"foreignKey:MachineID;constraint:OnDelete:RESTRICT" json:"machine,omitempty"`
}

// ─── OperationalResource ──────────────────────────────────────────────────────

type OperationalResource struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ResourceID  uint      `gorm:"not null;uniqueIndex:uq_res_opres" json:"resource_id"`
	Name        string    `gorm:"not null;uniqueIndex:uq_res_opres" json:"name"`
	Available   float64   `gorm:"not null" json:"available"` // DISP(R)
	CostPerUnit float64   `gorm:"not null" json:"cost_per_unit"` // CR(R)
	Resource    Resource  `gorm:"foreignKey:ResourceID;constraint:OnDelete:CASCADE" json:"-"`
}

// ─── ProductOperationalResource ──────────────────────────────────────────────

type ProductOperationalResource struct {
	ID                    uint                `gorm:"primaryKey" json:"id"`
	ProductID             uint                `gorm:"not null;uniqueIndex:uq_prod_opres" json:"product_id"`
	OperationalResourceID uint                `gorm:"not null;uniqueIndex:uq_prod_opres" json:"operational_resource_id"`
	ConsumptionPerBatch   float64             `gorm:"not null" json:"consumption_per_batch"` // CM(I,R)
	Product               Product             `gorm:"foreignKey:ProductID;constraint:OnDelete:RESTRICT" json:"-"`
	OperationalResource   OperationalResource `gorm:"foreignKey:OperationalResourceID;constraint:OnDelete:RESTRICT" json:"operational_resource,omitempty"`
}

// ─── Stock ────────────────────────────────────────────────────────────────────

type Stock struct {
	ID          uint             `gorm:"primaryKey" json:"id"`
	Name        string           `gorm:"not null" json:"name"`
	Ingredients []StockIngredient `gorm:"foreignKey:StockID" json:"ingredients,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type StockIngredient struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	StockID           uint       `gorm:"not null;uniqueIndex:uq_stock_ing" json:"stock_id"`
	IngredientID      uint       `gorm:"not null;uniqueIndex:uq_stock_ing" json:"ingredient_id"`
	QuantityAvailable float64    `gorm:"not null" json:"quantity_available"`
	Stock             Stock      `gorm:"foreignKey:StockID;constraint:OnDelete:CASCADE" json:"-"`
	Ingredient        Ingredient `gorm:"foreignKey:IngredientID;constraint:OnDelete:RESTRICT" json:"ingredient,omitempty"`
}

// ─── Resource ─────────────────────────────────────────────────────────────────

type Resource struct {
	ID                   uint                  `gorm:"primaryKey" json:"id"`
	Name                 string                `gorm:"not null" json:"name"`
	Machines             []ResourceMachine     `gorm:"foreignKey:ResourceID" json:"machines,omitempty"`
	OperationalResources []OperationalResource `gorm:"foreignKey:ResourceID" json:"operational_resources,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

type ResourceMachine struct {
	ID             uint     `gorm:"primaryKey" json:"id"`
	ResourceID     uint     `gorm:"not null;uniqueIndex:uq_res_mach" json:"resource_id"`
	MachineID      uint     `gorm:"not null;uniqueIndex:uq_res_mach" json:"machine_id"`
	HoursAvailable float64  `gorm:"not null" json:"hours_available"`
	Resource       Resource `gorm:"foreignKey:ResourceID;constraint:OnDelete:CASCADE" json:"-"`
	Machine        Machine  `gorm:"foreignKey:MachineID;constraint:OnDelete:RESTRICT" json:"machine,omitempty"`
}

// ─── Optimization ─────────────────────────────────────────────────────────────

type OptimizationStatus string

const (
	StatusPending    OptimizationStatus = "pending"
	StatusProcessing OptimizationStatus = "processing"
	StatusDone       OptimizationStatus = "done"
	StatusError      OptimizationStatus = "error"
	StatusCancelled  OptimizationStatus = "cancelled"
)

type Optimization struct {
	ID            uint                 `gorm:"primaryKey" json:"id"`
	StockID       uint                 `gorm:"not null" json:"stock_id"`
	ResourceID    uint                 `gorm:"not null" json:"resource_id"`
	Status        OptimizationStatus   `gorm:"type:varchar(20);default:'pending'" json:"status"`
	JobID         string               `gorm:"not null;uniqueIndex" json:"job_id"`
	MaxProduction float64              `gorm:"not null;default:200" json:"max_production"` // M
	MinVariety    int                  `gorm:"not null;default:7" json:"min_variety"`       // PRO
	TotalProfit   float64              `json:"total_profit"`
	// InputSnapshot: foto congelada (JSON) de stock/resource/products usados al
	// resolver. Permite reproducir y comparar escenarios aunque después se editen
	// los singleton Stock/Resource. Null en corridas viejas previas a esta feature.
	InputSnapshot json.RawMessage      `gorm:"type:jsonb" json:"input_snapshot,omitempty"`
	Stock         Stock                `gorm:"foreignKey:StockID;constraint:OnDelete:RESTRICT" json:"stock,omitempty"`
	Resource      Resource             `gorm:"foreignKey:ResourceID;constraint:OnDelete:RESTRICT" json:"resource,omitempty"`
	Results       []OptimizationResult `gorm:"foreignKey:OptimizationID" json:"results,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	StartedAt     *time.Time           `json:"started_at,omitempty"`
	FinishedAt    *time.Time           `json:"finished_at,omitempty"`
}

type OptimizationResult struct {
	ID                uint         `gorm:"primaryKey" json:"id"`
	OptimizationID    uint         `gorm:"not null;index" json:"optimization_id"`
	ProductID         uint         `gorm:"not null" json:"product_id"`
	QuantityToProduce float64      `gorm:"not null" json:"quantity_to_produce"` // X(I)
	BatchActive       float64      `gorm:"not null;default:0" json:"batch_active"` // Y(I)
	VarietyFlag       float64      `gorm:"not null;default:0" json:"variety_flag"` // W(I)
	ExpectedProfit    float64      `gorm:"not null" json:"expected_profit"`
	Optimization      Optimization `gorm:"foreignKey:OptimizationID;constraint:OnDelete:CASCADE" json:"-"`
	Product           Product      `gorm:"foreignKey:ProductID;constraint:OnDelete:RESTRICT" json:"product,omitempty"`
}

// ─── LingoLog ─────────────────────────────────────────────────────────────────

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
