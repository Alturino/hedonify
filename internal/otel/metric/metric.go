package metric

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/Alturino/ecommerce/internal/constants"
)

func InitMetricProvider(c context.Context, endpoint string) (*metric.MeterProvider, error) {
	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "main InitMetricProvider").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing metricExporter").Logger()
	logger.Info().Msg("initializing metricExporter")
	metricExporter, err := otlpmetricgrpc.New(
		c,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		err = fmt.Errorf("failed to initializing metricExporter with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("initialized metricExporter")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing meterProvider").Logger()
	logger.Info().Msg("initializing meterProvider")
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(metricExporter, metric.WithInterval(5*time.Second)),
		),
	)
	logger.Info().Msg("initialized meterProvider")

	return meterProvider, nil
}
