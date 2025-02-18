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
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	commonOtel "github.com/Alturino/ecommerce/user/internal/common/otel"
	"github.com/Alturino/ecommerce/user/internal/controller"
	"github.com/Alturino/ecommerce/user/internal/service"
)

func RunUserService(c context.Context) {
	c, span := commonOtel.Tracer.Start(c, "RunUserService")
	defer span.End()

	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", constants.APP_USER_SERVICE)).
		With().
		Str(log.KEY_APP_NAME, constants.APP_USER_SERVICE).
		Str(log.KEY_TAG, "main RunUserService").
		Logger()

	logger = logger.With().Str(log.KEY_PROCESS, "initializing config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.InitConfig(c, constants.APP_USER_SERVICE)
	logger = logger.With().Any(log.KEY_CONFIG, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.StrictSlash(true)
	mux.Use(otelmux.Middleware(constants.APP_USER_SERVICE), middleware.Logging)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	otelShutdowns, err := otel.InitOtelSdk(c, constants.APP_USER_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing userService").Logger()
	logger.Info().Msg("initializing userService")
	queries := repository.New(db)
	userService := service.NewUserService(queries, cfg.Application, cache)
	logger.Info().Msg("initialized userService")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing userController").Logger()
	logger.Info().Msg("initializing userController")
	controller.AttachUserController(c, mux, userService)
	logger.Info().Msg("initialized userController")

	logger = logger.With().Str(log.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext: func(net.Listener) context.Context {
			lg := logger.With().
				Reset().
				Str(log.KEY_APP_NAME, constants.APP_USER_SERVICE).
				Logger()
			c = lg.WithContext(c)
			return c
		},
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(log.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(log.KEY_PROCESS, "shutdown server").Logger()
			err = fmt.Errorf("error=%w occured while server is running", err)

			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())

			c = logger.WithContext(c)
			if err := otel.ShutdownOtel(c, otelShutdowns); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)

				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())

			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(log.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")

	logger.Info().Msg("shutting down otel")
	c = logger.WithContext(c)
	err = otel.ShutdownOtel(c, otelShutdowns)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("shutting down http server")
	err = server.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("failed shutting down http server with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("shutdown http server")
}
