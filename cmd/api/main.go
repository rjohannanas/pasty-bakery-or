package main

// @title LINGO Bakery API
// @version 1.0
// @description Backend para la optimización de producción de una panadería usando LINGO.
// @contact.name Soporte Técnico
// @host localhost:8080
// @BasePath /api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"lingo-backend/internal/db"
	"lingo-backend/internal/handlers"
	"lingo-backend/internal/logger"
	"lingo-backend/internal/middleware"
	"lingo-backend/internal/queue"
	"lingo-backend/internal/worker"
	"lingo-backend/internal/ws"

	"github.com/gin-contrib/cors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "lingo-backend/docs"
)

func main() {
	// 1. Cargar Variables de Entorno
	if err := godotenv.Load(); err != nil {
		// No morimos si no hay .env (podrían estar en el sistema directamente)
		println("Advertencia: No se pudo cargar el archivo .env")
	}

	// 2. Inicializar Logger
	logFile := os.Getenv("LOG_FILE")
	if logFile == "" { logFile = "logs/backend.log" }
	if err := logger.Init(logFile); err != nil {
		panic("Error inicializando logger: " + err.Error())
	}

	logger.L.Info().Msg("🚀 Iniciando LINGO Bakery Backend...")

	// 3. Conectar a PostgreSQL
	dbURL := os.Getenv("DATABASE_URL")
	pg, err := db.Connect(dbURL)
	if err != nil {
		logger.L.Fatal().Err(err).Msg("Falla crítica conectando a Postgres")
	}

	// 4. Conectar a Redis
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisAddr := os.Getenv("REDIS_ADDR")
	rdb, err := queue.Connect(ctx, redisAddr)
	if err != nil {
		logger.L.Fatal().Err(err).Msg("Falla crítica conectando a Redis")
	}

	// 5. Inicializar WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// 6. Iniciar Worker en background (pasando el hub)
	go worker.Start(ctx, pg, rdb, hub)

	// 7. Configurar Router Gin
	r := gin.Default()

	// Middleware de CORS: permite todo por ahora para facilitar Lovable/Túneles
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "X-Api-Key")
	r.Use(cors.New(corsConfig))

	// Endpoints de salud
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "time": time.Now()})
	})

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	apiKey := os.Getenv("APP_API_KEY")
	if apiKey == "" {
		logger.L.Warn().Msg("⚠️  APP_API_KEY no está seteada: la API queda ABIERTA sin protección. Configurala en .env para producción.")
	}

	// WebSocket endpoint (auth vía query param, el navegador no manda headers custom en el handshake)
	if apiKey != "" {
		r.GET("/ws", middleware.APIKeyAuthWS(apiKey), handlers.WebSocketHandler(hub))
	} else {
		r.GET("/ws", handlers.WebSocketHandler(hub))
	}

	api := r.Group("/api")
	if apiKey != "" {
		api.Use(middleware.APIKeyAuth(apiKey))
	}
	{
		// Escenarios (contenedor / unidad de what-if)
		api.GET("/scenarios", handlers.ListScenarios(pg))
		api.POST("/scenarios", handlers.CreateScenario(pg))
		api.GET("/scenarios/:scenario_id", handlers.GetScenario(pg))
		api.PATCH("/scenarios/:scenario_id", handlers.UpdateScenario(pg))
		api.DELETE("/scenarios/:scenario_id", handlers.DeleteScenario(pg))
		api.POST("/scenarios/:scenario_id/clone", handlers.CloneScenario(pg))
		api.POST("/scenarios/:scenario_id/optimize", handlers.OptimizeScenario(pg, rdb))

		// Productos del escenario + recetas (Q/T/CM)
		api.GET("/scenarios/:scenario_id/products", handlers.ListProducts(pg))
		api.POST("/scenarios/:scenario_id/products", handlers.CreateProduct(pg))
		api.GET("/scenarios/:scenario_id/products/:product_id", handlers.GetProduct(pg))
		api.PATCH("/scenarios/:scenario_id/products/:product_id", handlers.UpdateProduct(pg))
		api.DELETE("/scenarios/:scenario_id/products/:product_id", handlers.DeleteProduct(pg))
		api.PUT("/scenarios/:scenario_id/products/:product_id/ingredients/:ingredient_id", handlers.UpsertProductIngredient(pg))
		api.DELETE("/scenarios/:scenario_id/products/:product_id/ingredients/:ingredient_id", handlers.RemoveProductIngredient(pg))
		api.PUT("/scenarios/:scenario_id/products/:product_id/machines/:machine_id", handlers.UpsertProductMachine(pg))
		api.DELETE("/scenarios/:scenario_id/products/:product_id/machines/:machine_id", handlers.RemoveProductMachine(pg))
		api.PUT("/scenarios/:scenario_id/products/:product_id/operational-resources/:opres_id", handlers.UpsertProductOperationalResource(pg))
		api.DELETE("/scenarios/:scenario_id/products/:product_id/operational-resources/:opres_id", handlers.RemoveProductOperationalResource(pg))

		// Insumos del escenario
		api.GET("/scenarios/:scenario_id/ingredients", handlers.ListIngredients(pg))
		api.POST("/scenarios/:scenario_id/ingredients", handlers.CreateIngredient(pg))
		api.GET("/scenarios/:scenario_id/ingredients/:ingredient_id", handlers.GetIngredient(pg))
		api.PATCH("/scenarios/:scenario_id/ingredients/:ingredient_id", handlers.UpdateIngredient(pg))
		api.DELETE("/scenarios/:scenario_id/ingredients/:ingredient_id", handlers.DeleteIngredient(pg))

		// Máquinas del escenario
		api.GET("/scenarios/:scenario_id/machines", handlers.ListMachines(pg))
		api.POST("/scenarios/:scenario_id/machines", handlers.CreateMachine(pg))
		api.GET("/scenarios/:scenario_id/machines/:machine_id", handlers.GetMachine(pg))
		api.PATCH("/scenarios/:scenario_id/machines/:machine_id", handlers.UpdateMachine(pg))
		api.DELETE("/scenarios/:scenario_id/machines/:machine_id", handlers.DeleteMachine(pg))

		// Recursos operativos del escenario
		api.GET("/scenarios/:scenario_id/operational-resources", handlers.ListOperationalResources(pg))
		api.POST("/scenarios/:scenario_id/operational-resources", handlers.CreateOperationalResource(pg))
		api.GET("/scenarios/:scenario_id/operational-resources/:opres_id", handlers.GetOperationalResource(pg))
		api.PATCH("/scenarios/:scenario_id/operational-resources/:opres_id", handlers.UpdateOperationalResource(pg))
		api.DELETE("/scenarios/:scenario_id/operational-resources/:opres_id", handlers.DeleteOperationalResource(pg))

		// Corridas
		api.GET("/optimizations/:job_id", handlers.GetJobStatus(pg, rdb))
		api.GET("/optimize/queue/status", handlers.GetQueueStatus(rdb))
		api.GET("/results", handlers.ListOptimizations(pg))
		api.GET("/results/:id", handlers.GetOptimizationResult(pg))

		// Logs
		api.GET("/logs/lingo", handlers.ListLingoLogs(pg))
		api.GET("/logs/lingo/:job_id", handlers.GetLingoLogsByJobID(pg))
	}

	// 8. Servidor con Graceful Shutdown
	bindAddr := os.Getenv("BIND_ADDR")
	if bindAddr == "" {
		bindAddr = "0.0.0.0:8080"
	}

	server := &http.Server{
		Addr:    bindAddr,
		Handler: r,
	}

	logger.L.Info().Msgf("📡 API escuchando en %s", bindAddr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L.Fatal().Err(err).Msg("Falla al iniciar servidor")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.L.Info().Msg("Cerrando servidor...")

	cancel() // Detiene el worker
	time.Sleep(1 * time.Second) // Tiempo para que el worker limpie
}
