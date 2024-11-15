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
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/middleware"
	"github.com/Alturino/ecommerce/internal/otel"
)

func runNotificationService(c context.Context) {
	appName := "notification-service"

	pLogger := log.InitLogger(fmt.Sprintf("/var/log/%s.log", appName))
	logger := pLogger.With().
		Str(log.KeyAppName, appName).
		Logger()
	c = logger.WithContext(c)

	logger.Info().Str(log.KeyProcess, "InitOtelSdk").Msg("initalizing otel sdk")
	shutdownFuncs, err := otel.InitOtelSdk(c, common.AppNotificationService)
	if err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "InitOtelSdk").
			Msgf("failed initalizing otel sdk with error=%s", err.Error())
	}
	logger.Info().Str(log.KeyProcess, "InitOtelSdk").Msg("initalized otel sdk")

	logger.Info().Str(log.KeyProcess, "Start Server").Msg("initalizing router")
	router := mux.NewRouter()
	logger.Info().Str(log.KeyProcess, "Start Server").Msg("initalized router")
	router.Use(middleware.Logging)
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", "localhost", 8080),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      router,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	logger.Info().Str(log.KeyProcess, "Start Server").Msg("initalized router")

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
		logger.Info().Str(log.KeyProcess, "Shutdown Server").Msg("shutdown server")
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
