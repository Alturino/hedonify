package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/Alturino/ecommerce/internal/log"
)

func main() {
	pLogger := log.InitLogger("./ecommerce.log")
	logger := pLogger.With().Str(log.KeyTag, "main").Logger()

	logger.Info().Msg("adding listener for SIGINT and SIGTERM")
	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info().Msg("added listener for SIGINT and SIGTERM")

	c = logger.WithContext(c)

	// logger.Info().Msg("initalizing otel sdk")
	// shutdownFuncs, err := otel.InitOtelSdk(c)
	// if err != nil {
	// 	logger.Error().
	// 		Err(err).
	// 		Str(log.KeyTag, "main").
	// 		Msgf("failed initalizing otel sdk with error=%s", err.Error())
	// }
	// logger.Info().Msg("initalized otel sdk")

	router := mux.NewRouter()
	server := http.Server{
		Addr:         fmt.Sprintf("%s:%d", "localhost", 8080),
		BaseContext:  func(net.Listener) context.Context { return c },
		Handler:      router,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}

	go func() {
		logger.Info().
			Str(log.KeyProcess, "Start server").
			Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().
				Err(err).
				Str(log.KeyProcess, "Shutdown server").
				Msg("shutting down server")
		}
	}()

	<-c.Done()
	logger.Info().
		Str(log.KeyProcess, "Shutdown Server").
		Msg("received interuption signal shutting down")
	if err := server.Shutdown(c); err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "Shutdown Server").
			Msgf("shutting down server with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "Shutdown Server").
		Msg("shutdown server")
}
