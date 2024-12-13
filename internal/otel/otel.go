package otel

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Alturino/ecommerce/internal/config"
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

func InitOtelSdk(
	c context.Context,
	serviceName string,
	config config.Otel,
) (shutdownFuncs []ShutdownFunc, err error) {
	requestId := uuid.NewString()
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "main InitOtelSdk").
		Str(log.KeyRequestID, requestId).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing otel propagator").Logger()
	logger.Info().Msg("initializing otel propagator")
	propagator := newPropagator()
	otel.SetTextMapPropagator(propagator)
	logger.Info().Msg("initialized otel propagator")

	logger = logger.With().Str(log.KeyProcess, "initializing otel tracerProvider").Logger()
	logger.Info().Msg("initializing otel tracerProvider")
	c = logger.WithContext(c)
	tracerProvider, err := trace.InitTracerProvider(
		c,
		fmt.Sprintf("%s:%d", config.Host, config.Port),
		serviceName,
	)
	if err != nil {
		err = fmt.Errorf("failed initializing otel tracerProvider with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	otel.SetTracerProvider(tracerProvider)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	logger.Info().Msg("initialized otel tracerProvider")

	logger = logger.With().Str(log.KeyProcess, "initializing meterProvider").Logger()
	logger.Info().Msg("initializing meterProvider")
	c = logger.WithContext(c)
	meterProvider, err := metric.InitMetricProvider(
		c,
		fmt.Sprintf("%s:%d", config.Host, config.Port),
	)
	if err != nil {
		err = fmt.Errorf("failed initializing otel meterProvider with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return shutdownFuncs, err
	}
	otel.SetMeterProvider(meterProvider)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	logger.Info().Msg("initialized meterProvider")

	return shutdownFuncs, nil
}

func ShutdownOtel(c context.Context, shutdownFuncs []ShutdownFunc) error {
	var wg sync.WaitGroup
	var err error
	for _, shutdown := range shutdownFuncs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if newErr := shutdown(c); newErr != nil {
				err = errors.Join(newErr)
			}
		}()
	}
	wg.Wait()
	return err
}
