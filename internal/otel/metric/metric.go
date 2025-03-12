package metric

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/Alturino/ecommerce/internal/constants"
)

func InitMetricProvider(
	c context.Context,
	endpoint string,
	res *resource.Resource,
) (*metric.MeterProvider, error) {
	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "main InitMetricProvider").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing otlpMetricExporter").Logger()
	logger.Info().Msg("initializing otlpMetricExporter")
	otlpMetricExporter, err := otlpmetricgrpc.New(
		c,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		err = fmt.Errorf("failed to initializing otlpMetricExporter with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("initialized otlpMetricExporter")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing meterProvider").Logger()
	logger.Info().Msg("initializing meterProvider")
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(
			metric.NewPeriodicReader(otlpMetricExporter, metric.WithInterval(2*time.Second)),
		),
	)
	logger.Info().Msg("initialized meterProvider")

	return meterProvider, nil
}
