package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	commonOtel "github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/controller"
	"github.com/Alturino/ecommerce/cart/internal/repository"
	"github.com/Alturino/ecommerce/cart/internal/service"
	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
)

func RunCartService(c context.Context) {
	requestId := uuid.NewString()

	c, span := commonOtel.Tracer.Start(
		c,
		"RunCartService",
		trace.WithAttributes(attribute.String(log.KeyRequestID, requestId)),
	)
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyAppName, constants.AppCartService).
		Str(log.KeyTag, "main RunCartService").
		Str(log.KeyRequestID, requestId).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "init config").Logger()
	logger.Info().Msg("initializing config")
	c = logger.WithContext(c)
	cfg := config.InitConfig(c, constants.AppCartService)
	logger = logger.With().Any(log.KeyConfig, cfg).Logger()
	logger.Info().Msg("initialized config")

	logger = logger.With().Str(log.KeyProcess, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Use(otelmux.Middleware(constants.AppCartService), middleware.Logging, middleware.Auth)
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(log.KeyProcess, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	otelShutdowns, err := otel.InitOtelSdk(c, constants.AppCartService, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

		return
	}
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(log.KeyProcess, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	c = logger.WithContext(c)
	db := infra.NewDatabaseClient(c, cfg.Database)
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(log.KeyProcess, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	c = logger.WithContext(c)
	cache := infra.NewCacheClient(c, cfg.Cache)
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(log.KeyProcess, "initializing cart service").Logger()
	logger.Info().Msg("initializing cart service")
	queries := repository.New(db)
	httpClient := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   5 * time.Second,
	}
	cartService := service.NewCartService(db, queries, cache, &httpClient)
	logger.Info().Msg("initialized cart service")

	logger = logger.With().Str(log.KeyProcess, "initializing cart controller").Logger()
	logger.Info().Msg("initializing cart controller")
	controller.AttachCartController(mux, &cartService)
	logger.Info().Msg("initialized cart controller")

	logger = logger.With().Str(log.KeyProcess, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	httpServer := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(log.KeyProcess, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", httpServer.Addr)

		logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("error=%w occured while server is running", err)

			commonErrors.HandleError(err, logger, span)
			logger.Error().Err(err).Msg(err.Error())

			c = logger.WithContext(c)
			if err := otel.ShutdownOtel(c, otelShutdowns); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)

				commonErrors.HandleError(err, logger, span)
				logger.Error().Err(err).Msg(err.Error())

				return
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	<-c.Done()
	logger = logger.With().Str(log.KeyProcess, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")

	logger.Info().Msg("shutting down http server")
	c = logger.WithContext(c)
	err = httpServer.Shutdown(c)
	if err != nil {
		err = fmt.Errorf("filed shutting down http server with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

	}
	logger.Info().Msg("shutdown http server")

	logger.Info().Msg("shutting down otel")
	c = logger.WithContext(c)
	err = otel.ShutdownOtel(c, otelShutdowns)
	if err != nil {
		err = fmt.Errorf("failed shutting down otel with error=%w", err)

		commonErrors.HandleError(err, logger, span)
		logger.Error().Err(err).Msg(err.Error())

	}
	logger.Info().Msg("shutdown otel")

	logger.Info().Msg("server completely shutdown")
}
