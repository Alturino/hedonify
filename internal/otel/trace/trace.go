package trace

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/Alturino/ecommerce/internal/common"
	"github.com/Alturino/ecommerce/internal/log"
)

var Tracer = otel.Tracer(common.MainEcommerce)

func InitTracerProvider(
	c context.Context,
	endpoint, serviceName string,
) (*trace.TracerProvider, error) {
	logger := zerolog.Ctx(c).With().
		Str(log.KeyTag, "InitTracerProvider").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "Init TraceExporter").
		Msg("initializing traceExporter")
	traceExporter, err := otlptracegrpc.New(
		c,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "main").
			Msgf("failed creating traceExporter with error=%s", err.Error())
		return nil, err
	}
	logger.Info().
		Str(log.KeyProcess, "Init TraceExporter").
		Msg("initialized traceExporter")

	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initializing traceProvider")

	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initializing traceRes")
	traceRes, err := resource.New(c, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "main").
			Msgf("failed creating traceRes with error=%s", err.Error())
		return nil, err
	}
	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initalized traceRes")
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter, trace.WithBatchTimeout(5*time.Second)),
		trace.WithResource(traceRes),
	)
	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initialized traceProvider")

	return traceProvider, nil
}
