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
	"github.com/Alturino/ecommerce/product/internal/controller"
	"github.com/Alturino/ecommerce/product/internal/repository"
	"github.com/Alturino/ecommerce/product/internal/service"
)

func StartProductService(c context.Context) {
	logger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", common.AppProductService)).
		With().
		Str(log.KeyAppName, common.AppOrderService).
		Str(log.KeyTag, "main StartProductService").
		Logger()
	c = logger.WithContext(c)

	logger.Info().
		Str(log.KeyProcess, "InitOtelSdk").
		Msg("initalizing otel sdk")
	shutdownFuncs, err := otel.InitOtelSdk(c, common.AppProductService)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "InitOtelSdk").
			Msgf("failed initalizing otel sdk with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "InitOtelSdk").
		Msg("initalized otel sdk")

	logger.Info().
		Str(log.KeyProcess, "init config").
		Msg("initializing config")
	cfg := config.InitConfig(c, common.AppProductService)
	logger = logger.With().
		Any(log.KeyConfig, cfg).
		Logger()
	c = logger.WithContext(c)
	logger.Info().
		Str(log.KeyProcess, "init config").
		Msg("initialized config")

	logger.Info().
		Str(log.KeyProcess, "init database").
		Msg("initializing database")
	db := database.NewDatabaseClient(c, cfg.Database)
	logger.Info().
		Str(log.KeyProcess, "init database").
		Msg("initialized database")

	logger.Info().
		Str(log.KeyProcess, "initializing productService").
		Msg("initializing productService")
	queries := repository.New(db)
	productService := service.NewProductService(queries)
	logger.Info().
		Str(log.KeyProcess, "initializing productService").
		Msg("initialized productService")

	logger.Info().
		Str(log.KeyProcess, "Start Server").
		Msg("initalizing router")
	mux := mux.NewRouter()
	logger.Info().
		Str(log.KeyProcess, "Start Server").
		Msg("initalized router")
	mux.Use(middleware.Logging)

	controller.AttachProductController(mux, &productService)
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().
		Str(log.KeyProcess, "Start Server").
		Msg("initalized router")

	go func() {
		logger.Info().
			Str(log.KeyProcess, "Start Server").
			Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().
				Err(err).
				Str(log.KeyProcess, "Shutdown server").
				Msgf("error=%s occured while server is running", err.Error())
			if err := otel.ShutdownOtel(c, shutdownFuncs); err != nil {
				logger.Error().
					Err(err).
					Str(log.KeyProcess, "Shutdown server").
					Msgf("failed shutting down otel with error=%s", err.Error())
			}
		}
		logger.Info().
			Str(log.KeyProcess, "Shutdown Server").
			Msg("shutdown server")
	}()

	<-c.Done()
	logger.Info().
		Str(log.KeyProcess, "Shutdown Server").
		Msg("received interuption signal shutting down")
	err = server.Shutdown(c)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "Shutdown Server").
			Msgf("failed shutting down server with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "Shutdown Server").
		Msg("shutting down otel")
	err = otel.ShutdownOtel(c, shutdownFuncs)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "Shutdown Server").
			Msgf("failed shutting down otel with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "Shutdown Server").
		Msg("shutdown otel")

	logger.Info().
		Str(log.KeyProcess, "Shutdown Server").
		Msg("shutdown server")
}
