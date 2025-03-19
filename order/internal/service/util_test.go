package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	testRedis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/order/pkg/request"
	"github.com/Alturino/ecommerce/order/pkg/response"
	productRes "github.com/Alturino/ecommerce/product/pkg/response"
)

type (
	inputFunc    func() (req []request.CreateOrder, orderId []uuid.UUID, orderItemIds []uuid.UUID, product []productRes.Product, users []repository.User)
	setupFunc    func(context.Context, ...string) (*redis.Client, *pgxpool.Pool, *postgres.PostgresContainer, *testRedis.RedisContainer, *repository.Queries, *OrderService)
	teardownFunc func(*redis.Client, *pgxpool.Pool, *postgres.PostgresContainer, *testRedis.RedisContainer)
	expectedFunc func(orderIds []uuid.UUID, orderItemIds []uuid.UUID, products []productRes.Product, users []repository.User) map[string]response.Order
)

func setup(t *testing.T) setupFunc {
	return func(c context.Context, seedPaths ...string) (*redis.Client, *pgxpool.Pool, *postgres.PostgresContainer, *testRedis.RedisContainer, *repository.Queries, *OrderService) {
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
			postgres.WithInitScripts(
				append(
					[]string{
						filepath.Join("migrations", "20241118072912_create_table_products.up.sql"),
						filepath.Join("migrations", "20241112144824_create_table_users.up.sql"),
						filepath.Join("migrations", "20241125115439_create_table_orders.up.sql"),
						filepath.Join("migrations", "20241119141816_create_table_carts.up.sql"),
						filepath.Join("seed", "users.seed.sql"),
					},
					seedPaths...)...,
			),
		)
		if err != nil {
			t.Fatalf("failed running postgres container with error: %s", err)
		}

		pgConnStr, err := pgContainer.ConnectionString(c)
		if err != nil {
			t.Fatalf("failed getting postgres connection string with error: %s", err)
		}

		pgConfig, err := pgxpool.ParseConfig(pgConnStr)
		if err != nil {
			t.Fatalf("failed Parsing Config pgconfig with error: %s", err)
		}

		pool, err := pgxpool.NewWithConfig(c, pgConfig)
		if err != nil {
			t.Fatalf("failed ping postgres pool with error: %s", err)
		}

		if err = pool.Ping(c); err != nil {
			t.Fatalf("failed ping postgres pool with error: %s", err)
		}

		redisContainer, err := testRedis.Run(
			c,
			"redis:7.4.2-alpine3.21",
			testRedis.WithLogLevel(testRedis.LogLevelVerbose),
		)
		if err != nil {
			t.Fatalf("failed running redis container with error: %s", err)
		}

		redisConnStr, err := redisContainer.ConnectionString(c)
		if err != nil {
			t.Fatalf("failed getting redis connection string with error: %s", err)
		}

		redisOpt, err := redis.ParseURL(redisConnStr)
		if err != nil {
			t.Fatalf("failed getting redis connection string with error: %s", err)
		}

		redisClient := redis.NewClient(redisOpt)
		if err = redisClient.Ping(c).Err(); err != nil {
			t.Fatalf("failed ping redis client with error: %s", err)
		}

		queries := repository.New(pool)
		orderService := NewOrderService(pool, queries, redisClient)
		return redisClient, pool, pgContainer, redisContainer, queries, orderService
	}
}

func teardown(t *testing.T) teardownFunc {
	return func(redis *redis.Client, pool *pgxpool.Pool, pgContainer *postgres.PostgresContainer, redisContainer *testRedis.RedisContainer) {
		defer func() {
			redis.Close()
			pool.Close()
			if err := testcontainers.TerminateContainer(pgContainer); err != nil {
				t.Fatalf("failed to terminate container: %s", err)
			}
			if err := testcontainers.TerminateContainer(redisContainer); err != nil {
				t.Fatalf("failed to terminate container: %s", err)
			}
		}()
	}
}

type BatchOrderCreationTest struct {
	name                      string
	input                     inputFunc
	setup                     setupFunc
	seedPath                  []string
	teardown                  teardownFunc
	expected                  expectedFunc
	expectedRemainingQuantity int
	expectedErr               error
}
