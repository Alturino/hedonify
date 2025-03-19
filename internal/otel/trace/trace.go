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

	"github.com/Alturino/ecommerce/internal/constants"
)

func InitTracerProvider(
	c context.Context,
	endpoint, serviceName string,
	res *resource.Resource,
) (*trace.TracerProvider, error) {
		logger := zerolog.Ctx(c).
		With().
    Ctx(c).
		Str(constants.KEY_TAG, "main InitTracerProvider").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing traceExporter").Logger()
	logger.Trace().Msg("initializing traceExporter")
	traceExporter, err := otlptracegrpc.New(
		c,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		err = fmt.Errorf("failed creating traceExporter with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("initialized traceExporter")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing traceRes").Logger()
	logger.Trace().Msg("initializing traceRes")
	traceRes, err := resource.Merge(
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
		res,
	)
	if err != nil {
		err = fmt.Errorf("failed creating traceRes with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("initialized traceRes")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing traceProvider").Logger()
	logger.Trace().Msg("initializing traceProvider")
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter, trace.WithBatchTimeout(5*time.Second)),
		trace.WithResource(traceRes),
	)
	logger.Info().Msg("initialized traceProvider")

	return traceProvider, nil
}
