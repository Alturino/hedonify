package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/Alturino/ecommerce/internal/common"
	"github.com/Alturino/ecommerce/internal/config"
	database "github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/order/internal/controller"
	"github.com/Alturino/ecommerce/order/internal/repository"
	"github.com/Alturino/ecommerce/order/internal/service"
)

func RunOrderService(c context.Context) {
	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", common.AppCartService)).
		With().
		Str(log.KeyAppName, common.AppCartService).
		Str(log.KeyTag, "main RunCartService").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "init config").
		Msg("initializing config")
	cfg := config.InitConfig(c, common.AppCartService)
	logger = logger.With().
		Any(log.KeyConfig, cfg).
		Logger()
	c = logger.WithContext(c)
	logger.Info().
		Str(log.KeyProcess, "init config").
		Msg("initialized config")

	logger.Info().
		Str(log.KeyProcess, "start server").
		Msg("initalizing router")
	mux := mux.NewRouter()
	logger.Info().
		Str(log.KeyProcess, "start server").
		Msg("initalized router")

	logger.Info().
		Str(log.KeyProcess, "start server").
		Msg("attaching middleware")
	mux.Use(middleware.Logging)
	logger.Info().
		Str(log.KeyProcess, "start server").
		Msg("attached middleware")

	logger.Info().
		Str(log.KeyProcess, "InitOtelSdk").
		Msg("initalizing otel sdk")
	otelShutdowns, err := otel.InitOtelSdk(c, common.AppCartService)
	if err != nil {
		logger.Error().Err(err).Stack().
			Str(log.KeyProcess, "InitOtelSdk").
			Msgf("failed initalizing otel sdk with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "InitOtelSdk").
		Msg("initalized otel sdk")

	logger.Info().
		Str(log.KeyProcess, "init database").
		Msg("initializing database")
	db := database.NewDatabaseClient(c, cfg.Database)
	logger.Info().
		Str(log.KeyProcess, "init database").
		Msg("initialized database")

	logger.Info().
		Str(log.KeyProcess, "initializing cartService").
		Msg("initializing cartService")
	queries := repository.New(db)
	orderService := service.NewOrderService(db, queries)
	logger.Info().
		Str(log.KeyProcess, "initializing cartService").
		Msg("initialized cartService")

	logger.Info().
		Str(log.KeyProcess, "initializing userController").
		Msg("initializing userController")
	controller.AttachOrderController(mux, &orderService)
	logger.Info().
		Str(log.KeyProcess, "initialized userController").
		Msg("initialized userController")

	logger.Info().
		Str(log.KeyProcess, "start server").
		Msg("initalizing server")
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().
		Str(log.KeyProcess, "start server").
		Msg("initialized server")

	go func() {
		logger.Info().
			Str(log.KeyProcess, "start server").
			Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Stack().
				Str(log.KeyProcess, "Shutdown server").
				Msgf("error=%s occured while server is running", err.Error())
			if err := otel.ShutdownOtel(c, otelShutdowns); err != nil {
				logger.Error().Err(err).Stack().
					Str(log.KeyProcess, "Shutdown server").
					Msgf("failed shutting down otel with error=%s", err.Error())
			}
		}
		logger.Info().
			Str(log.KeyProcess, "shutdown server").
			Msg("shutdown server")
	}()

	<-c.Done()
	logger.Info().
		Str(log.KeyProcess, "shutdown server").
		Msg("received interuption signal shutting down")

	logger.Info().
		Str(log.KeyProcess, "shutdown server").
		Msg("shutting down otel")
	err = otel.ShutdownOtel(c, otelShutdowns)
	if err != nil {
		logger.Error().Err(err).Stack().
			Str(log.KeyProcess, "shutdown server").
			Msgf("failed shutting down otel with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "shutdown server").
		Msg("shutdown otel")

	logger.Info().
		Str(log.KeyProcess, "shutdown server").
		Msg("shutting down http server")
	err = server.Shutdown(c)
	if err != nil {
		logger.Error().Err(err).Stack().
			Str(log.KeyProcess, "shutdown server").
			Msgf("failed shutting down http server with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "shutdown server").
		Msg("shutdown http server")
}
