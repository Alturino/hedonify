package service

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	testRedis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/order/pkg/request"
)

func TestBatchOrderCreation(t *testing.T) {
	c := context.Background()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout})
	c = logger.WithContext(c)

	log.Println("starting postgres container")
	pgContainer, err := postgres.Run(
		c,
		"postgres:16.6-alpine3.21",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_DB":       "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_PORT":     "5432",
			"POSTGRES_USER":     "postgres",
		}),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithDatabase("postgres"),
		postgres.BasicWaitStrategies(),
		postgres.WithInitScripts("../../../seed/products.seed.sql", "../../../seed/users.seed.sql"),
	)
	if err != nil {
		t.Fatalf("failed running postgres container with error: %s", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()
	log.Println("started postgres container")

	log.Println("starting redis container")
	redisContainer, err := testRedis.Run(
		c,
		"redis/redis-stack:6.2.6-v17",
		testRedis.WithSnapshotting(10, 1),
		testRedis.WithLogLevel(testRedis.LogLevelVerbose),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithOccurrence(1).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed running redis container with error: %s", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(redisContainer); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()
	log.Println("started redis container")

	connStr, err := pgContainer.ConnectionString(c)
	if err != nil {
		t.Fatalf("failed getting postgres connection string with error: %s", err)
	}

	pool, err := pgxpool.New(c, connStr)
	if err != nil {
		t.Fatalf("failed creating pgx pool with error: %s", err)
	}

	connStr, err = redisContainer.ConnectionString(c)
	if err != nil {
		t.Fatalf("failed getting redis connection string with error: %s", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: connStr})

	filepath.Join("seed", "users.seed.sql")

	orderService := NewOrderService(pool, repository.New(pool), redisClient)
	err = orderService.BatchCreateOrder(c, []request.CreateOrder{})
	if err != nil {
		t.Fatalf("failed batch create order with error: %s", err)
	}
}
