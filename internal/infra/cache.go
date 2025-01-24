package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
)

var (
	cacheOnce sync.Once
	cache     *redis.Client
)

func NewCacheClient(
	c context.Context,
	config config.Cache,
) *redis.Client {
	c, span := otel.Tracer.Start(c, "main NewCacheClient")
	defer span.End()
	cacheOnce.Do(func() {
		logger := zerolog.Ctx(c).
			With().
			Str(log.KeyTag, "main NewCacheClient").
			Logger()

		logger = logger.With().Str(log.KeyProcess, "initializing redis client").Logger()
		logger.Info().Msg("initializing redis client")
		cache = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
			Password: config.Password,
			DB:       config.Database,
		})
		logger.Info().Msg("initialized redis client")

		logger = logger.With().Str(log.KeyProcess, "initializing redis otel tracing").Logger()
		logger.Info().Msg("initializing redis otel tracing")
		err := redisotel.InstrumentTracing(cache, redisotel.WithAttributes(semconv.DBSystemRedis))
		if err != nil {
			err = fmt.Errorf("failed initializing otel redis tracing with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("initialized redis otel tracing")

		logger = logger.With().Str(log.KeyProcess, "initializing redis otel metric").Logger()
		logger.Info().Msg("initializing redis otel metric")
		err = redisotel.InstrumentMetrics(cache, redisotel.WithAttributes(semconv.DBSystemRedis))
		if err != nil {
			err = fmt.Errorf("failed initializing otel redis metric with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("initialized redis otel metric")

		logger = logger.With().Str(log.KeyProcess, "pinging connection to redis").Logger()
		logger.Info().Msg("pinging connection to redis")
		err = cache.Ping(c).Err()
		if err != nil {
			err = fmt.Errorf("failed to pinging to redis with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("pinged connection to redis")

		logger.Info().Msg("initialized cache")
	})
	return cache
}
