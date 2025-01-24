package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/Alturino/ecommerce/internal/common/errors"
	inOtel "github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/log"
)

func InitTracerProvider(
	c context.Context,
	endpoint, serviceName string,
) (*trace.TracerProvider, error) {
	c, span := inOtel.Tracer.Start(c, "InitTracerProvider")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "main InitTracerProvider").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing traceExporter").Logger()
	logger.Info().Msg("initializing traceExporter")
	traceExporter, err := otlptracegrpc.New(
		c,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		err = fmt.Errorf("failed creating traceExporter with error=%w", err)
		errors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("initialized traceExporter")

	logger = logger.With().Str(log.KeyProcess, "initializing traceRes").Logger()
	logger.Info().Msg("initializing traceRes")
	traceRes, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		err = fmt.Errorf("failed creating traceRes with error=%w", err)
		errors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("initialized traceRes")

	logger = logger.With().Str(log.KeyProcess, "initializing traceProvider").Logger()
	logger.Info().Msg("initializing traceProvider")
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter, trace.WithBatchTimeout(5*time.Second)),
		trace.WithResource(traceRes),
	)
	logger.Info().Msg("initialized traceProvider")

	return traceProvider, nil
}
