package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"lingo-backend/internal/models"
)

// Productos scoped al escenario (/scenarios/:scenario_id/products).

func ListProducts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var products []models.Product
		if err := db.
			Preload("Ingredients.Ingredient").Preload("Machines.Machine").
			Preload("OperationalResources.OperationalResource").
			Where("scenario_id = ?", c.Param("scenario_id")).Order("id").Find(&products).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, products)
	}
}

func GetProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p models.Product
		if err := db.
			Preload("Ingredients.Ingredient").Preload("Machines.Machine").
			Preload("OperationalResources.OperationalResource").
			Where("scenario_id = ?", c.Param("scenario_id")).First(&p, c.Param("product_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

// productInput: numéricos SIN binding:required (invariante A2 — 0 es válido).
type productInput struct {
	Name      *string  `json:"name"`
	SalePrice *float64 `json:"sale_price"`
	Demand    *float64 `json:"demand"`
	MinBatch  *float64 `json:"min_batch"`
	MaxBatch  *float64 `json:"max_batch"`
}

func CreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok {
			return
		}
		if !requireDraft(c, s) {
			return
		}
		var in productInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name == nil || *in.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name es obligatorio"})
			return
		}
		p := models.Product{ScenarioID: s.ID, Name: *in.Name}
		applyProductInput(&p, in)
		if err := db.Create(&p).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se pudo crear (¿nombre duplicado en el escenario?)"})
			return
		}
		c.JSON(http.StatusCreated, p)
	}
}

func UpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok {
			return
		}
		if !requireDraft(c, s) {
			return
		}
		var p models.Product
		if err := db.Where("scenario_id = ?", s.ID).First(&p, c.Param("product_id")).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}
		var in productInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Name != nil {
			p.Name = *in.Name
		}
		applyProductInput(&p, in)
		if err := db.Save(&p).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

// DeleteProduct: cascadea la receta por FK; nunca bloquea (ADR 0004).
func DeleteProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok {
			return
		}
		if !requireDraft(c, s) {
			return
		}
		if err := db.Where("scenario_id = ?", s.ID).Delete(&models.Product{}, c.Param("product_id")).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

func applyProductInput(p *models.Product, in productInput) {
	if in.SalePrice != nil {
		p.SalePrice = *in.SalePrice
	}
	if in.Demand != nil {
		p.Demand = *in.Demand
	}
	if in.MinBatch != nil {
		p.MinBatch = *in.MinBatch
	}
	if in.MaxBatch != nil {
		p.MaxBatch = *in.MaxBatch
	}
}

// ─── Recetas (Q / T / CM) ───────────────────────────────────────────────────────

// productDraftGuard valida escenario draft + que el producto exista en él.
func productDraftGuard(db *gorm.DB, c *gin.Context) (*models.Scenario, bool) {
	s, ok := getScenario(db, c)
	if !ok {
		return nil, false
	}
	if !requireDraft(c, s) {
		return nil, false
	}
	var cnt int64
	db.Model(&models.Product{}).Where("scenario_id = ? AND id = ?", s.ID, c.Param("product_id")).Count(&cnt)
	if cnt == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
		return nil, false
	}
	return s, true
}

func UpsertProductIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := productDraftGuard(db, c)
		if !ok {
			return
		}
		var in struct {
			Quantity float64 `json:"quantity"` // Q — 0 válido
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		row := models.ProductIngredient{}
		err := db.Where("product_id = ? AND ingredient_id = ?", c.Param("product_id"), c.Param("ingredient_id")).First(&row).Error
		if err == gorm.ErrRecordNotFound {
			row = models.ProductIngredient{ScenarioID: s.ID, ProductID: parseUint(c.Param("product_id")), IngredientID: parseUint(c.Param("ingredient_id")), Quantity: in.Quantity}
			err = db.Create(&row).Error
		} else if err == nil {
			row.Quantity = in.Quantity
			err = db.Save(&row).Error
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}

func RemoveProductIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := productDraftGuard(db, c); !ok {
			return
		}
		db.Where("product_id = ? AND ingredient_id = ?", c.Param("product_id"), c.Param("ingredient_id")).Delete(&models.ProductIngredient{})
		c.JSON(http.StatusNoContent, nil)
	}
}

func UpsertProductMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := productDraftGuard(db, c)
		if !ok {
			return
		}
		var in struct {
			MinutesPerUnit float64 `json:"minutes_per_unit"` // T
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		row := models.ProductMachine{}
		err := db.Where("product_id = ? AND machine_id = ?", c.Param("product_id"), c.Param("machine_id")).First(&row).Error
		if err == gorm.ErrRecordNotFound {
			row = models.ProductMachine{ScenarioID: s.ID, ProductID: parseUint(c.Param("product_id")), MachineID: parseUint(c.Param("machine_id")), MinutesPerUnit: in.MinutesPerUnit}
			err = db.Create(&row).Error
		} else if err == nil {
			row.MinutesPerUnit = in.MinutesPerUnit
			err = db.Save(&row).Error
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}

func RemoveProductMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := productDraftGuard(db, c); !ok {
			return
		}
		db.Where("product_id = ? AND machine_id = ?", c.Param("product_id"), c.Param("machine_id")).Delete(&models.ProductMachine{})
		c.JSON(http.StatusNoContent, nil)
	}
}

func UpsertProductOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := productDraftGuard(db, c)
		if !ok {
			return
		}
		var in struct {
			ConsumptionPerBatch float64 `json:"consumption_per_batch"` // CM
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		row := models.ProductOperationalResource{}
		err := db.Where("product_id = ? AND operational_resource_id = ?", c.Param("product_id"), c.Param("opres_id")).First(&row).Error
		if err == gorm.ErrRecordNotFound {
			row = models.ProductOperationalResource{ScenarioID: s.ID, ProductID: parseUint(c.Param("product_id")), OperationalResourceID: parseUint(c.Param("opres_id")), ConsumptionPerBatch: in.ConsumptionPerBatch}
			err = db.Create(&row).Error
		} else if err == nil {
			row.ConsumptionPerBatch = in.ConsumptionPerBatch
			err = db.Save(&row).Error
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}

func RemoveProductOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := productDraftGuard(db, c); !ok {
			return
		}
		db.Where("product_id = ? AND operational_resource_id = ?", c.Param("product_id"), c.Param("opres_id")).Delete(&models.ProductOperationalResource{})
		c.JSON(http.StatusNoContent, nil)
	}
}
