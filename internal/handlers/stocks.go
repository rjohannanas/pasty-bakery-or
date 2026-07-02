package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

// GetDefaultStock devuelve el único Stock del sistema (lo crea si no existe).
// El negocio maneja un solo inventario diario, no varios stocks paralelos.
func GetDefaultStock(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var stock models.Stock
		err := db.Preload("Ingredients.Ingredient").Order("id asc").First(&stock).Error
		if err == gorm.ErrRecordNotFound {
			stock = models.Stock{Name: "Stock Diario"}
			if err := db.Create(&stock).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			db.Preload("Ingredients.Ingredient").First(&stock, stock.ID)
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stock)
	}
}

func ListStocks(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var stocks []models.Stock
		if err := db.Find(&stocks).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stocks)
	}
}

func GetStock(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var stock models.Stock
		if err := db.Preload("Ingredients.Ingredient").First(&stock, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stock no encontrado"})
			return
		}
		c.JSON(http.StatusOK, stock)
	}
}

func CreateStock(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name        string `json:"name" binding:"required"`
			Ingredients []struct {
				IngredientID      uint    `json:"ingredient_id" binding:"required"`
				QuantityAvailable float64 `json:"quantity_available" binding:"required"`
			} `json:"ingredients"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		stock := models.Stock{Name: input.Name}
		
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&stock).Error; err != nil {
				return err
			}
			for _, ing := range input.Ingredients {
				si := models.StockIngredient{
					StockID:           stock.ID,
					IngredientID:      ing.IngredientID,
					QuantityAvailable: ing.QuantityAvailable,
				}
				if err := tx.Create(&si).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		db.Preload("Ingredients.Ingredient").First(&stock, stock.ID)
		c.JSON(http.StatusCreated, stock)
	}
}

func UpdateStock(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var stock models.Stock
		if err := db.First(&stock, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stock no encontrado"})
			return
		}

		var input struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.Name != "" {
			stock.Name = input.Name
			db.Save(&stock)
		}

		c.JSON(http.StatusOK, stock)
	}
}

// UpsertStockIngredient crea o actualiza la cantidad disponible de un ingrediente
// en el stock diario (upsert porque el front edita cantidades día a día, no
// siempre existe la fila previa para ese ingrediente).
func UpsertStockIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		stockID := c.Param("id")
		ingredientID := c.Param("ingredient_id")
		var input struct {
			QuantityAvailable float64 `json:"quantity_available" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var stock models.Stock
		if err := db.First(&stock, stockID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Stock no encontrado"})
			return
		}
		ingID, err := strconv.ParseUint(ingredientID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ingredient_id inválido"})
			return
		}

		var si models.StockIngredient
		err = db.Where("stock_id = ? AND ingredient_id = ?", stockID, ingredientID).First(&si).Error
		if err == gorm.ErrRecordNotFound {
			si = models.StockIngredient{StockID: stock.ID, IngredientID: uint(ingID), QuantityAvailable: input.QuantityAvailable}
			if err := db.Create(&si).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			si.QuantityAvailable = input.QuantityAvailable
			if err := db.Save(&si).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, si)
	}
}

// RemoveStockIngredient desvincula un ingrediente del stock (no borra el
// Ingredient global, solo la fila de cantidad diaria).
func RemoveStockIngredient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		stockID := c.Param("id")
		ingredientID := c.Param("ingredient_id")
		if err := db.Where("stock_id = ? AND ingredient_id = ?", stockID, ingredientID).Delete(&models.StockIngredient{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

func DeleteStock(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		// ON DELETE CASCADE se encarga de stock_ingredients
		if err := db.Delete(&models.Stock{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}
