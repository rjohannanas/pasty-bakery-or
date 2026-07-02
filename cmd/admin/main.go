package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"lingo-backend/internal/queue"
)

func main() {
	godotenv.Load()
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	ctx := context.Background()
	rdb, err := queue.Connect(ctx, redisAddr)
	if err != nil {
		fmt.Printf("Error conectando a Redis: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "list":
		statusMap, err := rdb.GetAllJobsStatus(ctx)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("ID del Job\t\t\t\tEstado")
		fmt.Println("------------------------------------------------------------")
		for id, st := range statusMap {
			fmt.Printf("%s\t%s\n", id, st)
		}

	case "queue":
		pending, err := rdb.ListPending(ctx)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Jobs en cola (%d):\n", len(pending))
		for i, id := range pending {
			fmt.Printf("%d. %s\n", i+1, id)
		}

	case "cancel":
		if len(os.Args) < 3 {
			fmt.Println("Uso: admin cancel <job_id>")
			return
		}
		jobID := os.Args[2]
		if err := rdb.CancelJob(ctx, jobID); err != nil {
			fmt.Printf("Error cancelando job: %v\n", err)
		} else {
			fmt.Printf("Job %s cancelado (removido de cola si estaba pendiente)\n", jobID)
		}

	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("LINGO Bakery Admin CLI")
	fmt.Println("Uso:")
	fmt.Println("  admin list          - Listar todos los jobs recientes y su estado")
	fmt.Println("  admin queue         - Listar solo los jobs esperando en cola")
	fmt.Println("  admin cancel <id>   - Cancelar un job pendiente")
}
