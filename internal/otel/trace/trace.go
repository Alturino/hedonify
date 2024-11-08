package otel

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/Alturino/ecommerce/internal/log"
)

func initTrace(c context.Context, endpoint string) (*trace.TracerProvider, error) {
	plogger := zerolog.Ctx(c)
	logger := plogger.With().
		Str(log.KeyTag, "initTrace").
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
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter, trace.WithBatchTimeout(5*time.Second)),
	)
	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initialized traceProvider")

	return traceProvider, nil
}
