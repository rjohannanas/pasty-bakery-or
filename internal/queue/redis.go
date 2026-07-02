package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	listJobs    = "lingo:jobs"
	keyStatus   = "lingo:status:"
	keyRetries  = "lingo:retries:"
)

type Client struct {
	rdb *redis.Client
}

// Connect conecta a Redis y hace un ping.
func Connect(ctx context.Context, addr string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	for attempt := 1; attempt <= 5; attempt++ {
		err := rdb.Ping(ctx).Err()
		if err == nil {
			return &Client{rdb: rdb}, nil
		}
		fmt.Printf("[REDIS] intento %d/5 fallido: %v — reintentando en 3s...\n", attempt, err)
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("no se pudo conectar a redis tras 5 intentos")
}

// PushJob agrega un job a la cola y marca su estado como pending.
func (c *Client) PushJob(ctx context.Context, jobID string) error {
	pipe := c.rdb.TxPipeline()
	pipe.RPush(ctx, listJobs, jobID)
	pipe.Set(ctx, keyStatus+jobID, "pending", 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// PopJob saca un job de la cola bloqueando hasta que haya uno, o si algo sale mal (timeout del contexto p/e).
func (c *Client) PopJob(ctx context.Context) (string, error) {
	// 0 significa que bloquea indefinidamente hasta que exista un item o se cancele el contexto.
	res, err := c.rdb.BLPop(ctx, 0, listJobs).Result()
	if err != nil {
		return "", err
	}
	// res[0] is the key, res[1] is the value element
	if len(res) < 2 {
		return "", fmt.Errorf("bad BLPOP response")
	}
	return res[1], nil
}

// SetStatus actualiza el estado del job en redis.
func (c *Client) SetStatus(ctx context.Context, jobID, status string) error {
	return c.rdb.Set(ctx, keyStatus+jobID, status, 24*time.Hour).Err()
}

// GetStatus obtiene el estado de un job.
func (c *Client) GetStatus(ctx context.Context, jobID string) (string, error) {
	return c.rdb.Get(ctx, keyStatus+jobID).Result()
}

// CancelJob lo saca de la cola si está ahí, y le pone status cancelled.
// No aborta el worker proactivamente en esta versión si ya está corriendo, pero
// evita que se procese si está encolado.
func (c *Client) CancelJob(ctx context.Context, jobID string) error {
	pipe := c.rdb.TxPipeline()
	pipe.LRem(ctx, listJobs, 0, jobID)
	pipe.Set(ctx, keyStatus+jobID, "cancelled", 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// IncrementRetry count for a job.
func (c *Client) IncrementRetry(ctx context.Context, jobID string) (int, error) {
	val, err := c.rdb.Incr(ctx, keyRetries+jobID).Result()
	// TTL de retries para no llenar de basura
	c.rdb.Expire(ctx, keyRetries+jobID, 24*time.Hour)
	return int(val), err
}

// ListPending retorna los jobs actualmente en la cola esperando a ser procesados.
func (c *Client) ListPending(ctx context.Context) ([]string, error) {
	return c.rdb.LRange(ctx, listJobs, 0, -1).Result()
}

// GetAllJobsStatus retorna todos los jobs a los que les quede algo en status.
// Útil para listar los jobs recientes y su estado en el admin cli.
func (c *Client) GetAllJobsStatus(ctx context.Context) (map[string]string, error) {
	var cursor uint64
	var keys []string
	for {
		var batch []string
		var err error
		batch, cursor, err = c.rdb.Scan(ctx, cursor, keyStatus+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	
	results := make(map[string]string)
	for _, key := range keys {
		val, _ := c.rdb.Get(ctx, key).Result()
		jobID := key[len(keyStatus):]
		results[jobID] = val
	}
	return results, nil
}
