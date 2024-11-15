package metric

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/Alturino/ecommerce/internal/log"
)

func InitMetricProvider(c context.Context, endpoint string) (*metric.MeterProvider, error) {
	logger := zerolog.Ctx(c).With().
		Str(log.KeyTag, "initMetric").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "Init MetricExporter").
		Msg("initializing metricExporter")
	metricExporter, err := otlpmetricgrpc.New(
		c,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "Init MetricExporter").
			Msg("failed to initializing metricExporter")
		return nil, err
	}
	logger.Info().
		Str(log.KeyProcess, "Init MetricExporter").
		Msg("initialized metricExporter")

	logger.Info().
		Str(log.KeyProcess, "Init MetricProvider").
		Msg("initializing meterProvider")
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(metricExporter, metric.WithInterval(5*time.Second)),
		),
	)
	logger.Info().
		Str(log.KeyProcess, "Init MetricProvider").
		Msg("initialized meterProvider")

	return meterProvider, nil
}
