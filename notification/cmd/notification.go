package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RunNotificationService(c context.Context) {
	c, span := otel.Tracer.Start(c, "main RunNotificationService")
	defer span.End()

	cfg := config.Get(c, constants.APP_NOTIFICATION_SERVICE)

	logger := log.Get(filepath.Join("/var/log/", constants.APP_NOTIFICATION_SERVICE+".log"), cfg.Application).
		With().
		Str(constants.KEY_APP_NAME, constants.APP_NOTIFICATION_SERVICE).
		Str(constants.KEY_TAG, "main runNotificationService").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := otel.InitOtelSdk(c, constants.APP_NOTIFICATION_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)

		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Use(
		otelmux.Middleware(constants.APP_NOTIFICATION_SERVICE),
		middleware.Logging,
	)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
			err = fmt.Errorf("error=%w occured while server is running", err)

			otel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())

			logger.Info().Msg("shutting down otel")
			c = logger.WithContext(c)
			if err := otel.ShutdownOtel(c, shutdownFuncs); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)

				otel.RecordError(err, span)
				logger.Error().Err(err).Msg(err.Error())

			}
			logger.Info().Msg("shutdown otel")
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")

	logger.Info().Msg("shutting down http server")
	c = logger.WithContext(c)
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down server with error=%w", err)

		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("shutdown down http server")

	logger = logger.With().Str(constants.KEY_PROCESS, "shutting down otel").Logger()
	logger.Info().Msg("shutting down otel")
	c = logger.WithContext(c)
	err = otel.ShutdownOtel(c, shutdownFuncs)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)

		otel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("server completely shutdown")
}
