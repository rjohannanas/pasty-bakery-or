package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"lingo-backend/internal/logger"
	"lingo-backend/internal/models"
	"lingo-backend/internal/queue"
	"lingo-backend/internal/solver"
	"lingo-backend/internal/ws"
)

const maxRetries = 3

// Start arranca un worker infinito en background que procesa jobs de la cola.
func Start(ctx context.Context, db *gorm.DB, q *queue.Client, hub *ws.Hub) {
	logger.L.Info().Msg("[WORKER] Worker iniciado, esperando jobs en cola...")

	recoverOrphanJobs(ctx, db, q)

	for {
		// Verificamos si se canceló el contexto principal
		select {
		case <-ctx.Done():
			logger.L.Info().Msg("[WORKER] Apagando worker.")
			return
		default:
		}

		// Bloquea hasta que aparezca un job en Redis
		jobID, err := q.PopJob(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}
			logger.L.Error().Err(err).Msg("[WORKER] Error al hacer PopJob de la cola, reintentando en 2s")
			time.Sleep(2 * time.Second)
			continue
		}

		// Si el status cambió (por ejemplo a cancelled via admin CLI), no lo procesamos
		status, _ := q.GetStatus(ctx, jobID)
		if status == string(models.StatusCancelled) {
			logger.L.Info().Str("job_id", jobID).Msg("[WORKER] Job ignorado porque estaba cancelado.")
			continue
		}

		logger.L.Info().Str("job_id", jobID).Msg("[WORKER] Job desencolado. Iniciando procesamiento.")
		
		processJob(ctx, db, q, hub, jobID)
	}
}

func processJob(ctx context.Context, db *gorm.DB, q *queue.Client, hub *ws.Hub, jobID string) {
	// 1. Marcar como processing y guardar started_at
	now := time.Now()
	q.SetStatus(ctx, jobID, string(models.StatusProcessing))
	broadcastStatus(hub, jobID, string(models.StatusProcessing))
	
	if err := db.Model(&models.Optimization{}).Where("job_id = ?", jobID).Updates(map[string]interface{}{
		"status":     models.StatusProcessing,
		"started_at": &now,
	}).Error; err != nil {
		logger.L.Warn().Err(err).Str("job_id", jobID).Msg("[WORKER] No se pudo actualizar a processing en postgres (continúa)")
	}

	var opt models.Optimization
	if err := db.Where("job_id = ?", jobID).First(&opt).Error; err != nil {
		handleJobError(ctx, db, q, hub, jobID, "No se encontró el job en DB para procesarlo")
		return
	}

	logger.L.Info().Str("job_id", jobID).Msg("[WORKER] Generando modelo...")

	// 2. Construir modelo
	modelStr, products, err := solver.BuildModel(db, &opt)
	if err != nil {
		handleJobError(ctx, db, q, hub, jobID, fmt.Sprintf("Error construyendo modelo: %v", err))
		return
	}

	// Snapshot de la config de entrada (best-effort: si falla, no aborta el job).
	if snap, snapErr := solver.BuildSnapshot(db, &opt); snapErr != nil {
		logger.L.Warn().Err(snapErr).Str("job_id", jobID).Msg("[WORKER] No se pudo guardar el snapshot de entrada")
	} else if err := db.Model(&models.Optimization{}).Where("job_id = ?", jobID).Update("input_snapshot", snap).Error; err != nil {
		logger.L.Warn().Err(err).Str("job_id", jobID).Msg("[WORKER] No se pudo persistir el snapshot de entrada")
	}

	// 3. Ejecutar LINGO
	startTime := time.Now()
	output, err := solver.RunLINGO(ctx, jobID, modelStr)
	duration := time.Since(startTime)

	// Guardar log de LINGO pase lo que pase
	lingoLog := &models.LingoLog{
		JobID:          jobID,
		OptimizationID: opt.ID,
		Level:          "info",
		Message:        "Ejecución terminada",
		ModelGenerated: modelStr,
		LingoOutput:    output,
		DurationMs:     duration.Milliseconds(),
	}

	if err != nil {
		lingoLog.Level = "error"
		lingoLog.Message = err.Error()
		logger.SaveLingoLog(db, lingoLog)
		handleJobError(ctx, db, q, hub, jobID, fmt.Sprintf("Error en el solver: %v", err))
		return
	}
	logger.SaveLingoLog(db, lingoLog)

	// 4. Parsear resultados
	lingoResult, err := solver.ParseOutput(output, products)
	if err != nil {
		handleJobError(ctx, db, q, hub, jobID, fmt.Sprintf("Error parseando output: %v", err))
		return
	}

	// 5. Guardar resultados en PostgreSQL
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, p := range products {
			x := lingoResult.X[p.ID]
			y := lingoResult.Y[p.ID]
			w := lingoResult.W[p.ID]

			if x <= 0.0001 { // Ignorar productos que no se producen
				continue
			}

			// Calcular ganancia esperada para este producto:
			// Profit = P*X - X*Sum(Q*CU) - Y*Sum(CM*CR)
			var ingCostSum float64
			for _, pi := range p.Ingredients {
				ingCostSum += pi.Quantity * pi.Ingredient.UnitCost
			}

			var resCostSum float64
			for _, por := range p.OperationalResources {
				resCostSum += por.ConsumptionPerBatch * por.OperationalResource.CostPerUnit
			}

			profit := (p.SalePrice * x) - (x * ingCostSum) - (y * resCostSum)

			res := models.OptimizationResult{
				OptimizationID:    opt.ID,
				ProductID:         p.ID,
				QuantityToProduce: x,
				BatchActive:       y,
				VarietyFlag:       w,
				ExpectedProfit:    profit,
			}
			if err := tx.Create(&res).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		handleJobError(ctx, db, q, hub, jobID, fmt.Sprintf("Error guardando resultados en DB: %v", err))
		return
	}

	// Finalizar exitosamente
	finishedTime := time.Now()
	q.SetStatus(ctx, jobID, string(models.StatusDone))
	broadcastStatus(hub, jobID, string(models.StatusDone))
	
	db.Model(&models.Optimization{}).Where("job_id = ?", jobID).Updates(map[string]interface{}{
		"status":       models.StatusDone,
		"total_profit": lingoResult.ObjectiveValue,
		"finished_at":  &finishedTime,
	})

	logger.L.Info().Str("job_id", jobID).Msg("[WORKER] Job finalizado con éxito.")
}

func handleJobError(ctx context.Context, db *gorm.DB, q *queue.Client, hub *ws.Hub, jobID, reason string) {
	logger.L.Error().Str("job_id", jobID).Msgf("[WORKER] Error de job: %s", reason)
	
	retries, _ := q.IncrementRetry(ctx, jobID)
	
	if retries <= maxRetries {
		logger.L.Info().Str("job_id", jobID).Int("retry", retries).Msg("[WORKER] Re-encolando job para reintento.")
		q.SetStatus(ctx, jobID, string(models.StatusPending))
		broadcastStatus(hub, jobID, string(models.StatusPending))
		q.PushJob(ctx, jobID) // Lo volvemos a meter al final de la cola
		
		// En Postgres vuelve a pending
		db.Model(&models.Optimization{}).Where("job_id = ?", jobID).Update("status", models.StatusPending)
	} else {
		logger.L.Error().Str("job_id", jobID).Msg("[WORKER] Máximo de reintentos alcanzado. Job marcado como error definitivo.")
		now := time.Now()
		q.SetStatus(ctx, jobID, string(models.StatusError))
		broadcastStatus(hub, jobID, string(models.StatusError))
		
		db.Model(&models.Optimization{}).Where("job_id = ?", jobID).Updates(map[string]interface{}{
			"status":      models.StatusError,
			"finished_at": now,
		})
	}
}

func broadcastStatus(hub *ws.Hub, jobID, status string) {
	msg, err := json.Marshal(map[string]string{"job_id": jobID, "status": status})
	if err != nil {
		logger.L.Error().Err(err).Msg("[WORKER] Error serializando mensaje de broadcast")
		return
	}
	hub.Broadcast(msg)
}

// recoverOrphanJobs busca jobs que hayan quedado en 'processing' cuando el servidor 
// se apagó bruscamente, y los re-encola como 'pending'.
func recoverOrphanJobs(ctx context.Context, db *gorm.DB, q *queue.Client) {
	// Leemos de redis a ver si había llaves en estado 'processing'
	allStatus, err := q.GetAllJobsStatus(ctx)
	if err != nil {
		logger.L.Warn().Err(err).Msg("[WORKER] Error al buscar orphans en Redis. Skippando recovery.")
		return
	}

	orphans := 0
	for jobID, st := range allStatus {
		if st == string(models.StatusProcessing) {
			logger.L.Warn().Str("job_id", jobID).Msg("[WORKER] Job huérfano encontrado (processing). Re-encolando.")
			q.SetStatus(ctx, jobID, string(models.StatusPending))
			db.Model(&models.Optimization{}).Where("job_id = ?", jobID).Update("status", models.StatusPending)
			q.PushJob(ctx, jobID)
			orphans++
		}
	}
	if orphans > 0 {
		logger.L.Info().Int("count", orphans).Msg("[WORKER] Se recuperaron jobs huérfanos.")
	}
}
