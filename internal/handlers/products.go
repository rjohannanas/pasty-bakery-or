package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

// ─── PRODUCT CRUD ───────────────────────────────────────────────────────────

// ListProducts lista todos los productos registrados
// @Summary Listar productos
// @Description Obtiene la lista completa de productos con sus precios y costos
// @Tags Products
// @Produce json
// @Success 200 {array} models.Product
// @Router /products [get]
func ListProducts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var products []models.Product
		if err := db.Find(&products).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, products)
	}
}

// GetProduct obtiene un producto con sus recetas de ingredientes y máquinas
// @Summary Obtener producto
// @Description Obtiene detalles de un producto, incluyendo ingredientes y máquinas asignadas
// @Tags Products
// @Produce json
// @Param id path int true "ID del producto"
// @Success 200 {object} models.Product
// @Router /products/{id} [get]
func GetProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Ingredients.Ingredient").Preload("Machines.Machine").Preload("OperationalResources.OperationalResource").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}
		c.JSON(http.StatusOK, product)
	}
}

// CreateProduct crea un nuevo producto base
// @Summary Crear producto
// @Description Registra un nuevo producto en el catálogo
// @Tags Products
// @Accept json
// @Produce json
// @Param input body models.Product true "Datos del producto"
// @Success 201 {object} models.Product
// @Router /products [post]
func CreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name      string  `json:"name" binding:"required"`
			SalePrice float64 `json:"sale_price" binding:"required"`
			Demand    float64 `json:"demand" binding:"required"`
			MinBatch  float64 `json:"min_batch"`
			MaxBatch  float64 `json:"max_batch"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		product := models.Product{
			Name:      input.Name,
			SalePrice: input.SalePrice,
			Demand:    input.Demand,
			MinBatch:  input.MinBatch,
			MaxBatch:  input.MaxBatch,
		}
		if err := db.Create(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, product)
	}
}

func UpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}
		var input struct {
			Name      string   `json:"name"`
			SalePrice float64  `json:"sale_price"`
			Demand    *float64 `json:"demand"`
			MinBatch  *float64 `json:"min_batch"`
			MaxBatch  *float64 `json:"max_batch"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if input.Name != "" { product.Name = input.Name }
		if input.SalePrice != 0 { product.SalePrice = input.SalePrice }
		if input.Demand != nil { product.Demand = *input.Demand }
		if input.MinBatch != nil { product.MinBatch = *input.MinBatch }
		if input.MaxBatch != nil { product.MaxBatch = *input.MaxBatch }

		if err := db.Save(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, product)
	}
}

func DeleteProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// La receta propia (ingredientes/máquinas/op.resources) cascadea al borrar.
		// Lo único que protege el borrado es el historial de optimizaciones.
		var results int64
		db.Model(&models.OptimizationResult{}).Where("product_id = ?", id).Count(&results)
		if results > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("El producto aparece en %d resultado(s) de optimización del historial y no se puede eliminar.", results),
			})
			return
		}

		if err := db.Delete(&models.Product{}, id).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No se puede eliminar el producto"})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

// ─── PRODUCT INGREDIENTS ─────────────────────────────────────────────────────

func AddProductIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		var input struct {
			IngredientID uint    `json:"ingredient_id" binding:"required"`
			Quantity     float64 `json:"quantity" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		// Validar que el producto existe
		var pID uint
		if err := db.Model(&models.Product{}).Select("id").First(&pID, productID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}

		pi := models.ProductIngredient{
			ProductID:    pID,
			IngredientID: input.IngredientID,
			Quantity:     input.Quantity,
		}
		if err := db.Create(&pi).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al asignar ingrediente (puede que ya esté asignado)"})
			return
		}
		c.JSON(http.StatusCreated, pi)
	}
}

func ListProductIngredients(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		var ingredients []models.ProductIngredient
		if err := db.Preload("Ingredient").Where("product_id = ?", productID).Find(&ingredients).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, ingredients)
	}
}

func UpdateProductIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		ingredientID := c.Param("ing_id")
		var input struct {
			Quantity float64 `json:"quantity" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Model(&models.ProductIngredient{}).Where("product_id = ? AND ingredient_id = ?", productID, ingredientID).Update("quantity", input.Quantity).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Cantidad actualizada"})
	}
}

func RemoveProductIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		ingredientID := c.Param("ing_id")
		if err := db.Where("product_id = ? AND ingredient_id = ?", productID, ingredientID).Delete(&models.ProductIngredient{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

// ─── PRODUCT MACHINES ────────────────────────────────────────────────────────

func AddProductMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		var input struct {
			MachineID      uint    `json:"machine_id" binding:"required"`
			MinutesPerUnit float64 `json:"minutes_per_unit" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		var pID uint
		if err := db.Model(&models.Product{}).Select("id").First(&pID, productID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}

		pm := models.ProductMachine{
			ProductID:      pID,
			MachineID:      input.MachineID,
			MinutesPerUnit: input.MinutesPerUnit,
		}
		if err := db.Create(&pm).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al asignar máquina"})
			return
		}
		c.JSON(http.StatusCreated, pm)
	}
}

func ListProductMachines(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		var machines []models.ProductMachine
		if err := db.Preload("Machine").Where("product_id = ?", productID).Find(&machines).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, machines)
	}
}

func UpdateProductMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		machineID := c.Param("machine_id")
		var input struct {
			MinutesPerUnit float64 `json:"minutes_per_unit" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Model(&models.ProductMachine{}).Where("product_id = ? AND machine_id = ?", productID, machineID).Update("minutes_per_unit", input.MinutesPerUnit).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Tiempo actualizado"})
	}
}

func RemoveProductMachine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		machineID := c.Param("machine_id")
		if err := db.Where("product_id = ? AND machine_id = ?", productID, machineID).Delete(&models.ProductMachine{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

// ─── PRODUCT OPERATIONAL RESOURCES ───────────────────────────────────────────

func AddProductOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		var input struct {
			OperationalResourceID uint    `json:"operational_resource_id" binding:"required"`
			ConsumptionPerBatch   float64 `json:"consumption_per_batch" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		var pID uint
		if err := db.Model(&models.Product{}).Select("id").First(&pID, productID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Producto no encontrado"})
			return
		}

		por := models.ProductOperationalResource{
			ProductID:             pID,
			OperationalResourceID: input.OperationalResourceID,
			ConsumptionPerBatch:   input.ConsumptionPerBatch,
		}
		if err := db.Create(&por).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al asignar recurso operativo"})
			return
		}
		c.JSON(http.StatusCreated, por)
	}
}

func ListProductOperationalResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		var resources []models.ProductOperationalResource
		if err := db.Preload("OperationalResource").Where("product_id = ?", productID).Find(&resources).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resources)
	}
}

func UpdateProductOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		opresID := c.Param("opres_id")
		var input struct {
			ConsumptionPerBatch float64 `json:"consumption_per_batch" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Model(&models.ProductOperationalResource{}).Where("product_id = ? AND operational_resource_id = ?", productID, opresID).Update("consumption_per_batch", input.ConsumptionPerBatch).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Consumo actualizado"})
	}
}

func RemoveProductOperationalResource(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")
		opresID := c.Param("opres_id")
		if err := db.Where("product_id = ? AND operational_resource_id = ?", productID, opresID).Delete(&models.ProductOperationalResource{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}


