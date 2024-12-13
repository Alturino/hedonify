package infra

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
)

func NewCacheClient(
	c context.Context,
	config config.Cache,
) *redis.Client {
	c, span := otel.Tracer.Start(c, "main NewCacheClient")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "main NewCacheClient").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing redis client").Logger()
	logger.Info().Msg("initializing redis client")
	redis := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.Database,
	})
	logger.Info().Msg("initialized redis client")

	logger = logger.With().Str(log.KeyProcess, "initializing redis otel tracing").Logger()
	logger.Info().Msg("initializing redis otel tracing")
	err := redisotel.InstrumentTracing(redis, redisotel.WithAttributes(semconv.DBSystemRedis))
	if err != nil {
		err = fmt.Errorf("failed initializing otel redis tracing with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("initialized redis otel tracing")

	logger = logger.With().Str(log.KeyProcess, "initializing redis otel metric").Logger()
	logger.Info().Msg("initializing redis otel metric")
	err = redisotel.InstrumentMetrics(redis, redisotel.WithAttributes(semconv.DBSystemRedis))
	if err != nil {
		err = fmt.Errorf("failed initializing otel redis metric with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("initialized redis otel metric")

	return redis
}
