package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"lingo-backend/internal/models"
)

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
