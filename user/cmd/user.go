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
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/user/internal/controller"
	"github.com/Alturino/ecommerce/user/internal/otel"
	"github.com/Alturino/ecommerce/user/internal/service"
)

func RunUserService(c context.Context) {
	c, span := otel.Tracer.Start(c, "RunUserService")
	defer span.End()

	cfg := config.Get(c, constants.APP_USER_SERVICE)

	logger := log.Get(filepath.Join("/var/log/", constants.APP_USER_SERVICE+".log"), cfg.Application).
		With().
		Str(constants.KEY_APP_NAME, constants.APP_USER_SERVICE).
		Str(constants.KEY_TAG, "main RunUserService").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Use(otelmux.Middleware(constants.APP_USER_SERVICE), middleware.Logging)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := inOtel.InitOtelSdk(c, constants.APP_USER_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	logger.Info().Msg("initialized otel sdk")
	defer func() {
		logger.Info().Msg("shutting down otel")
		c = logger.WithContext(c)
		err = inOtel.ShutdownOtel(c, shutdownFuncs)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("shutdown otel")
	}()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "closing database").Logger()
		logger.Info().Msg("closing database")
		db.Close()
		logger.Info().Msg("closed database")
	}()
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "closing cache").Logger()
		logger.Info().Msg("closing cache")
		err = cache.Close()
		if err != nil {
			err = fmt.Errorf("failed closing cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("closed cache")
	}()
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing userService").Logger()
	logger.Info().Msg("initializing userService")
	queries := repository.New(db)
	userService := service.NewUserService(queries, cfg.Application, cache)
	logger.Info().Msg("initialized userService")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing userController").Logger()
	logger.Info().Msg("initializing userController")
	controller.AttachUserController(c, mux, userService)
	logger.Info().Msg("initialized userController")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext: func(net.Listener) context.Context {
			lg := logger.With().
				Reset().
				Timestamp().
				Caller().
				Stack().
				Str(constants.KEY_APP_NAME, constants.APP_USER_SERVICE).
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
		logger = logger.With().Str(constants.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
			err = fmt.Errorf("error=%w occured while server is running", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			c = logger.WithContext(c)
			if err := inOtel.ShutdownOtel(c, shutdownFuncs); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				inOtel.RecordError(err, span)
				logger.Error().Err(err).Msg(err.Error())
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
}
