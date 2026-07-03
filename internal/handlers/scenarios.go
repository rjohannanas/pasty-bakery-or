package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"lingo-backend/internal/models"
	"lingo-backend/internal/queue"
)

// parseUint convierte un path param a uint (0 si inválido).
func parseUint(s string) uint {
	n, _ := strconv.ParseUint(s, 10, 64)
	return uint(n)
}

// ─── Helpers compartidos ───────────────────────────────────────────────────────

// getScenario carga el escenario del path param :scenario_id.
func getScenario(db *gorm.DB, c *gin.Context) (*models.Scenario, bool) {
	var s models.Scenario
	if err := db.First(&s, c.Param("scenario_id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Escenario no encontrado"})
		return nil, false
	}
	return &s, true
}

// requireDraft aborta si el escenario no es editable (invariante M3/L1).
func requireDraft(c *gin.Context, s *models.Scenario) bool {
	switch s.Status {
	case models.ScenarioFrozen:
		c.JSON(http.StatusConflict, gin.H{"error": "El escenario está congelado; clonalo para editar"})
		return false
	case models.ScenarioArchived:
		c.JSON(http.StatusConflict, gin.H{"error": "El escenario está archivado"})
		return false
	}
	return true
}

// ─── Scenario CRUD ──────────────────────────────────────────────────────────────

func ListScenarios(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := db.Order("created_at desc")
		if c.Query("include_archived") != "true" {
			q = q.Where("status <> ?", models.ScenarioArchived)
		}
		var scenarios []models.Scenario
		if err := q.Find(&scenarios).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, scenarios)
	}
}

func CreateScenario(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name          string  `json:"name" binding:"required"`
			Notes         string  `json:"notes"`
			MaxProduction float64 `json:"max_production"`
			MinVariety    int     `json:"min_variety"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		s := models.Scenario{
			Name:          input.Name,
			Notes:         input.Notes,
			Status:        models.ScenarioDraft,
			MaxProduction: defaultIfZero(input.MaxProduction, 200),
			MinVariety:    defaultIntIfZero(input.MinVariety, 7),
		}
		if err := db.Create(&s).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se pudo crear (¿nombre duplicado?)"})
			return
		}
		c.JSON(http.StatusCreated, s)
	}
}

func GetScenario(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var s models.Scenario
		err := db.
			Preload("Products.Ingredients.Ingredient").
			Preload("Products.Machines.Machine").
			Preload("Products.OperationalResources.OperationalResource").
			Preload("Ingredients").
			Preload("Machines").
			Preload("OperationalResources").
			First(&s, c.Param("scenario_id")).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Escenario no encontrado"})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

func UpdateScenario(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok {
			return
		}
		if !requireDraft(c, s) {
			return
		}
		var input struct {
			Name          *string  `json:"name"`
			Notes         *string  `json:"notes"`
			MaxProduction *float64 `json:"max_production"`
			MinVariety    *int     `json:"min_variety"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if input.Name != nil {
			s.Name = *input.Name
		}
		if input.Notes != nil {
			s.Notes = *input.Notes
		}
		if input.MaxProduction != nil {
			s.MaxProduction = *input.MaxProduction
		}
		if input.MinVariety != nil {
			s.MinVariety = *input.MinVariety
		}
		if err := db.Save(s).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, s)
	}
}

// DeleteScenario borra un draft sin corridas, o archiva uno con historia (L1).
func DeleteScenario(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok {
			return
		}
		var runs int64
		db.Model(&models.Optimization{}).Where("scenario_id = ?", s.ID).Count(&runs)
		if runs > 0 {
			// Tiene historia → archivar (SET NULL desvincula las corridas).
			db.Model(&models.Optimization{}).Where("scenario_id = ?", s.ID).Update("scenario_id", nil)
			s.Status = models.ScenarioArchived
			db.Save(s)
		} else {
			if err := db.Delete(&models.Scenario{}, s.ID).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

// CloneScenario copia un escenario completo a un nuevo draft (fork). Cada entidad
// hereda canonical_id (raíz) para comparación cruzada. Ver ADR 0003/0006.
func CloneScenario(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		src, ok := getScenario(db, c)
		if !ok {
			return
		}
		var input struct {
			Name string `json:"name"`
		}
		_ = c.ShouldBindJSON(&input)
		name := input.Name
		if name == "" {
			name = src.Name + " (copia)"
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			dst := models.Scenario{
				Name: name, Notes: src.Notes, Status: models.ScenarioDraft,
				ParentID: &src.ID, MaxProduction: src.MaxProduction, MinVariety: src.MinVariety,
			}
			if err := tx.Create(&dst).Error; err != nil {
				return err
			}

			// Copiar entidades; mapear id viejo → nuevo. canonical = raíz (o el id viejo).
			prodMap := map[uint]uint{}
			ingMap := map[uint]uint{}
			machMap := map[uint]uint{}
			opresMap := map[uint]uint{}

			var products []models.Product
			tx.Where("scenario_id = ?", src.ID).Find(&products)
			for _, p := range products {
				np := models.Product{
					ScenarioID: dst.ID, CanonicalID: rootCanonical(p.CanonicalID, p.ID),
					Name: p.Name, SalePrice: p.SalePrice, Demand: p.Demand,
					MinBatch: p.MinBatch, MaxBatch: p.MaxBatch,
				}
				if err := tx.Create(&np).Error; err != nil {
					return err
				}
				prodMap[p.ID] = np.ID
			}
			var ingredients []models.Ingredient
			tx.Where("scenario_id = ?", src.ID).Find(&ingredients)
			for _, ing := range ingredients {
				ni := models.Ingredient{
					ScenarioID: dst.ID, CanonicalID: rootCanonical(ing.CanonicalID, ing.ID),
					Name: ing.Name, Unit: ing.Unit, UnitCost: ing.UnitCost, StockAvailable: ing.StockAvailable,
				}
				if err := tx.Create(&ni).Error; err != nil {
					return err
				}
				ingMap[ing.ID] = ni.ID
			}
			var machines []models.Machine
			tx.Where("scenario_id = ?", src.ID).Find(&machines)
			for _, m := range machines {
				nm := models.Machine{
					ScenarioID: dst.ID, CanonicalID: rootCanonical(m.CanonicalID, m.ID),
					Name: m.Name, CapacityMinutes: m.CapacityMinutes,
				}
				if err := tx.Create(&nm).Error; err != nil {
					return err
				}
				machMap[m.ID] = nm.ID
			}
			var opres []models.OperationalResource
			tx.Where("scenario_id = ?", src.ID).Find(&opres)
			for _, o := range opres {
				no := models.OperationalResource{
					ScenarioID: dst.ID, CanonicalID: rootCanonical(o.CanonicalID, o.ID),
					Name: o.Name, Available: o.Available, CostPerUnit: o.CostPerUnit,
				}
				if err := tx.Create(&no).Error; err != nil {
					return err
				}
				opresMap[o.ID] = no.ID
			}

			// Recetas: remapear por los mapas (ids estables).
			var pis []models.ProductIngredient
			tx.Where("scenario_id = ?", src.ID).Find(&pis)
			for _, pi := range pis {
				if err := tx.Create(&models.ProductIngredient{
					ScenarioID: dst.ID, ProductID: prodMap[pi.ProductID],
					IngredientID: ingMap[pi.IngredientID], Quantity: pi.Quantity,
				}).Error; err != nil {
					return err
				}
			}
			var pms []models.ProductMachine
			tx.Where("scenario_id = ?", src.ID).Find(&pms)
			for _, pm := range pms {
				if err := tx.Create(&models.ProductMachine{
					ScenarioID: dst.ID, ProductID: prodMap[pm.ProductID],
					MachineID: machMap[pm.MachineID], MinutesPerUnit: pm.MinutesPerUnit,
				}).Error; err != nil {
					return err
				}
			}
			var pors []models.ProductOperationalResource
			tx.Where("scenario_id = ?", src.ID).Find(&pors)
			for _, por := range pors {
				if err := tx.Create(&models.ProductOperationalResource{
					ScenarioID: dst.ID, ProductID: prodMap[por.ProductID],
					OperationalResourceID: opresMap[por.OperationalResourceID],
					ConsumptionPerBatch:   por.ConsumptionPerBatch,
				}).Error; err != nil {
					return err
				}
			}
			c.Set("new_scenario_id", dst.ID)
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al clonar: " + err.Error()})
			return
		}
		newID, _ := c.Get("new_scenario_id")
		var out models.Scenario
		db.First(&out, newID)
		c.JSON(http.StatusCreated, out)
	}
}

// OptimizeScenario congela el escenario y encola la corrida (proceso en docs/04).
func OptimizeScenario(db *gorm.DB, q *queue.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, ok := getScenario(db, c)
		if !ok {
			return
		}
		if s.Status == models.ScenarioArchived {
			c.JSON(http.StatusConflict, gin.H{"error": "No se puede optimizar un escenario archivado"})
			return
		}

		// Invariante M4: al menos 1 producto con receta.
		var withRecipe int64
		db.Model(&models.ProductIngredient{}).Where("scenario_id = ?", s.ID).
			Distinct("product_id").Count(&withRecipe)
		if withRecipe == 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No hay productos configurados para optimizar"})
			return
		}

		var input struct {
			MaxProduction float64 `json:"max_production"`
			MinVariety    int     `json:"min_variety"`
		}
		_ = c.ShouldBindJSON(&input)
		maxP := defaultIfZero(input.MaxProduction, s.MaxProduction)
		minV := defaultIntIfZero(input.MinVariety, s.MinVariety)

		// Evitar corridas duplicadas simultáneas para el mismo escenario.
		var active int64
		db.Model(&models.Optimization{}).Where("scenario_id = ? AND status IN (?, ?)",
			s.ID, models.StatusPending, models.StatusProcessing).Count(&active)
		if active > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya hay una optimización en curso para este escenario"})
			return
		}

		// Congelar el escenario (la corrida referencia inputs inmutables).
		if s.Status == models.ScenarioDraft {
			db.Model(s).Update("status", models.ScenarioFrozen)
		}

		jobID := uuid.New().String()
		scenID := s.ID
		opt := models.Optimization{
			ScenarioID: &scenID, JobID: jobID, Status: models.StatusPending,
			MaxProduction: maxP, MinVariety: minV,
		}
		if err := db.Create(&opt).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := q.PushJob(c.Request.Context(), jobID); err != nil {
			db.Delete(&opt)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al encolar en Redis"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"job_id": jobID, "status": "pending"})
	}
}

// ─── utilidades ─────────────────────────────────────────────────────────────────

func defaultIfZero(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}
func defaultIntIfZero(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// rootCanonical devuelve la raíz de identidad: el canonical existente, o el id
// propio si es la primera vez (ancla raíz para comparación cruzada).
func rootCanonical(existing *uint, ownID uint) *uint {
	if existing != nil {
		return existing
	}
	id := ownID
	return &id
}
