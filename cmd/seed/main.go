package main

// Seed: ingesta el modelo base (MODELO_EXCEL.xlsx) como un Scenario{is_base:true}.
// El xlsx es el modelo LINGO fuente; cada tabla mapea 1:1 a una entidad.
//
// Uso:
//   go run ./cmd/seed              # siembra sobre una DB ya migrada y vacía de "Base"
//   go run ./cmd/seed --reset      # DROP de las tablas de la app y re-siembra (DESTRUCTIVO)
//   go run ./cmd/seed --file otro.xlsx
//
// Notas de conversión (ver docs/02-data-dictionary.md):
//   - CAP se guarda tal cual: xlsx y DB ambos en MINUTOS (capacity_minutes).
//   - X/Y/W del xlsx son la SOLUCIÓN de una corrida vieja, NO input → se ignoran.
//   - Solo se guardan celdas de receta ≠ 0 (una celda ausente = 0 en el solver).

import (
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"lingo-backend/internal/db"
	"lingo-backend/internal/logger"
	"lingo-backend/internal/models"
)

const sheet = "Hoja2"

func main() {
	reset := flag.Bool("reset", false, "DROP de las tablas de la app antes de sembrar (DESTRUCTIVO)")
	file := flag.String("file", "MODELO_EXCEL.xlsx", "ruta al xlsx fuente")
	flag.Parse()

	_ = godotenv.Load()
	if err := logger.Init(""); err != nil {
		panic(err)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.L.Fatal().Msg("DATABASE_URL vacía (¿falta .env?)")
	}

	// 1. Reset opcional: dropea las tablas de la app (las filas legacy bloquean
	//    AutoMigrate, así que hay que limpiar antes de que db.Connect migre).
	if *reset {
		raw, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
		if err != nil {
			logger.L.Fatal().Err(err).Msg("no se pudo abrir la DB para el reset")
		}
		drop := "DROP TABLE IF EXISTS optimization_results, lingo_logs, optimizations, " +
			"product_operational_resources, product_machines, product_ingredients, " +
			"operational_resources, machines, ingredients, products, scenarios CASCADE"
		if err := raw.Exec(drop).Error; err != nil {
			logger.L.Fatal().Err(err).Msg("falló el DROP de tablas")
		}
		logger.L.Warn().Msg("🗑️  tablas de la app dropeadas (--reset)")
	}

	// 2. Connect corre AutoMigrate + CHECKs + FK compuestas (esquema limpio).
	pg, err := db.Connect(dsn)
	if err != nil {
		logger.L.Fatal().Err(err).Msg("falló la migración")
	}

	// 3. Guard de idempotencia.
	var existing int64
	pg.Model(&models.Scenario{}).Where("name = ?", "Base").Count(&existing)
	if existing > 0 {
		logger.L.Fatal().Msg("ya existe un escenario 'Base'; usá --reset para re-sembrar")
	}

	// 4. Leer el xlsx.
	f, err := excelize.OpenFile(*file)
	if err != nil {
		logger.L.Fatal().Err(err).Msgf("no se pudo abrir %s", *file)
	}
	defer f.Close()
	rows, err := f.GetRows(sheet)
	if err != nil {
		logger.L.Fatal().Err(err).Msgf("no se pudo leer la hoja %s", sheet)
	}

	if err := seed(pg, rows); err != nil {
		logger.L.Fatal().Err(err).Msg("falló el seed")
	}
}

// seed inserta el escenario base y todo su dominio en una sola transacción.
func seed(pg *gorm.DB, rows [][]string) error {
	return pg.Transaction(func(tx *gorm.DB) error {
		// META=200 (M), VARIEDAD=7 (PRO): coinciden con los defaults del modelo.
		scen := models.Scenario{
			Name: "Base", IsBase: true, Status: models.ScenarioDraft,
			Notes: "Sembrado desde MODELO_EXCEL.xlsx", MaxProduction: 200, MinVariety: 7,
		}
		if err := tx.Create(&scen).Error; err != nil {
			return err
		}

		// TABLA 1: productos (filas 4..23). r[1]=ID r[2]=nombre r[3]=P r[4]=D r[5]=LI r[6]=LS.
		var prodIDs []uint
		for _, r := range rows[4:24] {
			p := models.Product{
				ScenarioID: scen.ID, Name: cell(r, 2),
				SalePrice: num(r, 3), Demand: num(r, 4), MinBatch: num(r, 5), MaxBatch: num(r, 6),
			}
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
			prodIDs = append(prodIDs, p.ID)
		}

		// TABLA 2: ingredientes (28..39). r[3]=C r[4]=IN r[5]=unidad.
		var ingIDs []uint
		for _, r := range rows[28:40] {
			ing := models.Ingredient{
				ScenarioID: scen.ID, Name: cell(r, 2), Unit: cell(r, 5),
				UnitCost: num(r, 3), StockAvailable: num(r, 4),
			}
			if err := tx.Create(&ing).Error; err != nil {
				return err
			}
			ingIDs = append(ingIDs, ing.ID)
		}

		// TABLA 3: máquinas (44..47). r[3]=CAP (minutos, sin conversión).
		var machIDs []uint
		for _, r := range rows[44:48] {
			m := models.Machine{ScenarioID: scen.ID, Name: cell(r, 2), CapacityMinutes: num(r, 3)}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			machIDs = append(machIDs, m.ID)
		}

		// TABLA 4: recursos operativos (52..54). r[3]=DISP r[4]=CR.
		var opresIDs []uint
		for _, r := range rows[52:55] {
			o := models.OperationalResource{
				ScenarioID: scen.ID, Name: cell(r, 2), Available: num(r, 3), CostPerUnit: num(r, 4),
			}
			if err := tx.Create(&o).Error; err != nil {
				return err
			}
			opresIDs = append(opresIDs, o.ID)
		}

		// TABLA 5: matriz Q (59..78), 20 filas × 12 cols. Celda ≠0 → ProductIngredient.
		for pos, r := range rows[59:79] {
			for j := range ingIDs {
				q := num(r, j+1)
				if q == 0 {
					continue
				}
				if err := tx.Create(&models.ProductIngredient{
					ScenarioID: scen.ID, ProductID: prodIDs[pos], IngredientID: ingIDs[j], Quantity: q,
				}).Error; err != nil {
					return err
				}
			}
		}

		// TABLA 6: matriz T (83..102), 20 × 4. Celda ≠0 → ProductMachine (minutos).
		for pos, r := range rows[83:103] {
			for k := range machIDs {
				t := num(r, k+1)
				if t == 0 {
					continue
				}
				if err := tx.Create(&models.ProductMachine{
					ScenarioID: scen.ID, ProductID: prodIDs[pos], MachineID: machIDs[k], MinutesPerUnit: t,
				}).Error; err != nil {
					return err
				}
			}
		}

		// TABLA 7: matriz CM (107..126), 20 × 3. Celda ≠0 → ProductOperationalResource.
		for pos, r := range rows[107:127] {
			for rr := range opresIDs {
				cm := num(r, rr+1)
				if cm == 0 {
					continue
				}
				if err := tx.Create(&models.ProductOperationalResource{
					ScenarioID: scen.ID, ProductID: prodIDs[pos], OperationalResourceID: opresIDs[rr],
					ConsumptionPerBatch: cm,
				}).Error; err != nil {
					return err
				}
			}
		}

		logger.L.Info().
			Int("productos", len(prodIDs)).Int("ingredientes", len(ingIDs)).
			Int("maquinas", len(machIDs)).Int("opres", len(opresIDs)).
			Uint("scenario_id", scen.ID).
			Msg("✅ escenario Base sembrado")
		return nil
	})
}

// cell devuelve la celda i (0-based) trimmeada, o "" si está fuera de rango.
func cell(r []string, i int) string {
	if i < len(r) {
		return strings.TrimSpace(r[i])
	}
	return ""
}

// num parsea la celda i como float; "" o no-numérico → 0.
func num(r []string, i int) float64 {
	v, err := strconv.ParseFloat(cell(r, i), 64)
	if err != nil {
		return 0
	}
	return v
}
