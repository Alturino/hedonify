package otel

import (
	"context"
	"errors"
	"sync"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/otel/metric"
	"github.com/Alturino/ecommerce/internal/otel/trace"
)

type ShutdownFunc func(context.Context) error

func newPropagator() propagation.TextMapPropagator {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	return propagator
}

func InitOtelSdk(c context.Context, serviceName string) (shutdownFuncs []ShutdownFunc, err error) {
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "InitOtelSdk").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "Init Propagator").
		Msg("initializing otel propagator")
	propagator := newPropagator()
	otel.SetTextMapPropagator(propagator)
	logger.Info().
		Str(log.KeyProcess, "Init Propagator").
		Msg("initialized otel propagator")

	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initializing otel tracerProvider")
	tracerProvider, err := trace.InitTracerProvider(c, "otel-collector:4317", serviceName)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "Init TracerProvider").
			Msgf("failed initializing otel tracerProvider with error=%s", err.Error())
		return nil, err
	}
	otel.SetTracerProvider(tracerProvider)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	logger.Info().
		Str(log.KeyProcess, "Init TracerProvider").
		Msg("initializing otel tracerProvider")

	logger.Info().
		Str(log.KeyProcess, "Init MeterProvider").
		Msg("initializing meterProvider")
	meterProvider, err := metric.InitMetricProvider(c, "otel-collector:4317")
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "Init MeterProvider").
			Msgf("failed initializing otel meterProvider with error=%s", err.Error())
		return shutdownFuncs, err
	}
	otel.SetMeterProvider(meterProvider)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	logger.Info().
		Str(log.KeyProcess, "Init MeterProvider").
		Msg("initialized meterProvider")

	return shutdownFuncs, nil
}

func ShutdownOtel(c context.Context, shutdownFuncs []ShutdownFunc) error {
	var wg sync.WaitGroup
	var err error
	for _, shutdown := range shutdownFuncs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := shutdown(c); err != nil {
				err = errors.Join(err)
			}
		}()
	}
	wg.Wait()
	return err
}
