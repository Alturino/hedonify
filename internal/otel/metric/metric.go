package metric

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
)

func InitMetric(c context.Context) (*metric.MeterProvider, error) {
	metricExporter, err := otlpmetricgrpc.New(
		c,
		otlpmetricgrpc.WithEndpoint("otel-collector:4317"),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		slog.ErrorContext(
			c,
			"failed creating metricExporter with error=%s", err.Error(),
			slog.String("process", "InitMetric"),
		)
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(metricExporter, metric.WithInterval(5*time.Second)),
		),
	)
	return meterProvider, nil
}
