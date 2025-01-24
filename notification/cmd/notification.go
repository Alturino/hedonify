package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	commonOtel "github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RunNotificationService(c context.Context) {
	c, span := commonOtel.Tracer.Start(c, "main RunNotificationService")
	defer span.End()

	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", constants.AppNotificationService)).
		With().
		Str(log.KeyAppName, constants.AppNotificationService).
		Str(log.KeyTag, "main runNotificationService").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.InitConfig(c, constants.AppNotificationService)
	logger = logger.With().Any(log.KeyConfig, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := otel.InitOtelSdk(c, constants.AppNotificationService, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KeyProcess, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Use(
		otelmux.Middleware(constants.AppNotificationService),
		middleware.Logging,
		middleware.Auth,
	)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KeyProcess, "initializing server").Logger()
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
		logger = logger.With().Str(log.KeyProcess, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
			err = fmt.Errorf("error=%w occured while server is running", err)

			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())

			logger.Info().Msg("shutting down otel")
			c = logger.WithContext(c)
			if err := otel.ShutdownOtel(c, shutdownFuncs); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)

				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())

			}
			logger.Info().Msg("shutdown otel")
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")

	logger.Info().Msg("shutting down http server")
	c = logger.WithContext(c)
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down server with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("shutdown down http server")

	logger = logger.With().Str(log.KeyProcess, "shutting down otel").Logger()
	logger.Info().Msg("shutting down otel")
	c = logger.WithContext(c)
	err = otel.ShutdownOtel(c, shutdownFuncs)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("server completely shutdown")
}
